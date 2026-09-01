# RFC 2865 - Remote Authentication Dial In User Service (RADIUS)

Supported for subscriber access. Every requirement this repository extracted from RFC 2865, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 82.8% | 24 of 29 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 17.2% | 5 of 29 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 29 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 29 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 42.5% | 37 of 87 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 29 | of 32 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 29 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 29 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported for subscriber access |
| Enrolment | Enrolled |
| Requirements | 32 |
| Gated MUST-level | 29 |
| Obligations that bind Ze | 29 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 96 |
| Tagged units | 87 |
| Recorded audit verdicts | 0 |
| Discrimination records | 37 |
| Summary | `rfc/short/rfc2865.md` |
| Requirement shard | `rfc/requirements/rfc2865.md` |
| RFC text | `rfc/full/rfc2865.txt` |

## Enrolment

Enrolled: Remote Authentication Dial In User Service (RADIUS), ze as a RADIUS client/NAS: ten MUST-level requirements, all met in internal/component/radius (and l2tp/authradius). Six carry positive+negative tags: 3-1 (packet length 20..4096), 3-3 (Response Authenticator = MD5 over Code, Identifier, Length, Request Authenticator, attributes, and secret), 3-4 (verify the Response Authenticator before trusting a response), 5.2-1 (User-Password hidden via the MD5-with-shared-secret XOR chain), 5-2 (an attribute Length is bounded to 255 octets), and 3-5 (accept a response only from the server the request was sent to). Four are {single-polarity: positive}: 3-2 (the Request Authenticator is 16 random octets), 2.5-1 (a retransmission reuses the same Identifier and Request Authenticator), 5-1 (an Access-Request includes a User-Name), and 5.2-2 (User-Password padded to a multiple of 16, capped at 128). Seven new tests bind the previously-untested behaviors.

## What the public ledger says

**Status:** Supported for subscriber access

**What the ledger says is covered:**

Access-Accept profile extraction, Filter-Id, Session-Timeout, Idle-Timeout, VSAs, pool selection.

**What the ledger says remains:**

Operator/admin login RADIUS (PAP) is a separate backend under `system/authentication/radius`; the profile attributes above are subscriber-access only.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 24 | one part of the gated population |
| Annotated instead of tested | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **29** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (24):** [`RFC2865-3-1`](#rfc2865-3-1), [`RFC2865-3-3`](#rfc2865-3-3), [`RFC2865-3-4`](#rfc2865-3-4), [`RFC2865-5-1`](#rfc2865-5-1), [`RFC2865-5.2-1`](#rfc2865-5.2-1), [`RFC2865-5-2`](#rfc2865-5-2), [`RFC2865-3-5`](#rfc2865-3-5), [`RFC2865-1.1-1`](#rfc2865-1.1-1), [`RFC2865-1.1-2`](#rfc2865-1.1-2), [`RFC2865-3-6`](#rfc2865-3-6), [`RFC2865-3-8`](#rfc2865-3-8), [`RFC2865-4.1-1`](#rfc2865-4.1-1), [`RFC2865-4.1-6`](#rfc2865-4.1-6), [`RFC2865-4.1-2`](#rfc2865-4.1-2), [`RFC2865-4.1-3`](#rfc2865-4.1-3), [`RFC2865-4.4-1`](#rfc2865-4.4-1), [`RFC2865-5-4`](#rfc2865-5-4), [`RFC2865-5-8`](#rfc2865-5-8), [`RFC2865-3-7`](#rfc2865-3-7), [`RFC2865-4.1-5`](#rfc2865-4.1-5), [`RFC2865-5-6`](#rfc2865-5-6), [`RFC2865-5-7`](#rfc2865-5-7), [`RFC2865-5.11-1`](#rfc2865-5.11-1), [`RFC2865-5.25-1`](#rfc2865-5.25-1)

**Annotated instead of tested (5):** [`RFC2865-3-2`](#rfc2865-3-2), [`RFC2865-2.5-1`](#rfc2865-2.5-1), [`RFC2865-5.2-2`](#rfc2865-5.2-2), [`RFC2865-4.1-4`](#rfc2865-4.1-4), [`RFC2865-5-5`](#rfc2865-5-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2865-3-1` | Packet Length MUST be between 20 and 4096 bytes (§3) | MUST | 3 - Packet Format | **positive:** `unit/verify` [`TestPacketRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L141). **negative:** `unit/verify` [`TestDecodeBadLength`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L193). **negative:** `unit/verify` [`TestDecodeTooLong`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L180). **negative:** `unit/verify` [`TestDecodeTooShort`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L171) |
| `RFC2865-3-2` | Request Authenticator MUST be 16 cryptographically random octets (§3) | MUST | 3 - Packet Format | **positive:** `unit/verify` [`TestRFC2865RequestAuthenticatorRandom`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_test.go#L40). **negative:** no negative test. **{single-polarity}:** the Request Authenticator is 16 octets from crypto/rand (internal/component/radius/packet.go:32) and there is no invalid-authenticator generation path to drive a negative |
| `RFC2865-3-3` | Response Authenticator MUST be MD5(Code+ID+Length+RequestAuth+Attributes+Secret) (§3) | MUST | 3 - Packet Format | **positive:** `unit/verify` [`TestRFC2865ResponseAuthenticatorMatchesTheFormula`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_response_auth_test.go#L58). **positive:** `unit/verify` [`TestResponseAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L206). **positive:** `unit/verify` [`TestVerifyResponseAuth`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L240). **negative:** `unit/verify` [`TestRFC2865ResponseAuthenticatorCoversEveryNamedField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_response_auth_test.go#L99). **negative:** `unit/verify` [`TestResponseAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L214). **negative:** `unit/verify` [`TestVerifyResponseAuth`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L246) |
| `RFC2865-3-4` | NAS MUST verify Response Authenticator before trusting a response (§3) | MUST | 3 - Packet Format | **positive:** `unit/verify` [`TestClientAuthenticatorVerify`](https://github.com/ze-software/ze/blob/main/internal/component/radius/client_test.go#L383). **negative:** `unit/verify` [`TestClientAuthenticatorVerify`](https://github.com/ze-software/ze/blob/main/internal/component/radius/client_test.go#L388) |
| `RFC2865-2.5-1` | A retransmitted request MUST use the same Identifier and Request Authenticator (§2.5) | MUST | 2.5 - Retransmission Hints | **positive:** `unit/verify` [`TestClientRetransmit`](https://github.com/ze-software/ze/blob/main/internal/component/radius/client_test.go#L194). **negative:** no negative test. **{single-polarity}:** a retransmission resends the identical pre-encoded request buffer (internal/component/radius/client.go:159), so the Identifier and Request Authenticator are unchanged by construction and there is no divergent-retransmit code path |
| `RFC2865-5-1` | The User-Name attribute MUST be sent in Access-Request packets if available (§5, sentence at §5.1) | MUST | 5 - Attributes | **positive:** `unit/verify` [`TestRFC2865AccessRequestUserName`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_test.go#L219). **positive:** `unit/verify` [`TestRFC2865SubscriberAccessRequestUserName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_test.go#L31). **negative:** `unit/verify` [`TestRFC2865AccessRequestOmitsAnUnavailableUserName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_access_request_shape_test.go#L75) |
| `RFC2865-5.2-1` | User-Password encoding MUST use MD5-based XOR chain: c[0] = p[0] XOR MD5(S+RA), c[i] = p[i] XOR MD5(S+c[i-1]) (§5.2) | MUST | 5.2 - User-Password | **positive:** `unit/verify` [`TestEncodeUserPassword`](https://github.com/ze-software/ze/blob/main/internal/component/radius/attr_test.go#L35). **positive:** `unit/verify` [`TestEncodeUserPasswordMultiBlock`](https://github.com/ze-software/ze/blob/main/internal/component/radius/attr_test.go#L97). **negative:** `unit/verify` [`TestRFC2865UserPasswordDependsOnSecret`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_test.go#L156) |
| `RFC2865-5.2-2` | User-Password MUST be padded to a multiple of 16 octets (max 128) (§5.2) | MUST | 5.2 - User-Password | **positive:** `unit/verify` [`TestEncodeUserPassword`](https://github.com/ze-software/ze/blob/main/internal/component/radius/attr_test.go#L17). **positive:** `unit/verify` [`TestEncodeUserPasswordEmpty`](https://github.com/ze-software/ze/blob/main/internal/component/radius/attr_test.go#L52). **positive:** `unit/verify` [`TestEncodeUserPasswordMultiBlock`](https://github.com/ze-software/ze/blob/main/internal/component/radius/attr_test.go#L75). **positive:** `unit/verify` [`TestRFC2865UserPasswordClamp`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_test.go#L175). **negative:** no negative test. **{single-polarity}:** the encoder always pads and clamps and never rejects (internal/component/radius/attr.go:18-26), so there is no reject path to drive a negative |
| `RFC2865-5-2` | Attribute length MUST NOT exceed 255 bytes (Type + Length + Value) (§5) | MUST | 5 - Attributes | **positive:** `unit/verify` [`TestRFC2865AttributeLengthBound`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_test.go#L78). **negative:** `unit/verify` [`TestRFC2865AttributeLengthBound`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_test.go#L94) |
| `RFC2865-3-5` | Only accept responses from the server address the request was sent to (§3) | MUST | 3 - Packet Format | **positive:** `unit/verify` [`TestClientExchangeAccept`](https://github.com/ze-software/ze/blob/main/internal/component/radius/client_test.go#L105). **positive:** `unit/verify` [`TestRFC2865ResponseSourceAddress`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_test.go#L139). **negative:** `unit/verify` [`TestRFC2865ResponseSourceAddress`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_test.go#L130) |
| `RFC2865-1.1-1` | A NAS that does not implement a given service MUST NOT implement the RADIUS attributes for that service (§1.1) | MUST NOT | 1.1 - Specification of Requirements | **positive:** `unit/verify` [`TestAdminAccessRequestCarriesNoUnofferedServiceAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L38). **positive:** `unit/verify` [`TestRFC2869DictionaryCoversTheServicesZeOffers`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2869_unoffered_service_attributes_test.go#L99). **negative:** `unit/verify` [`TestRFC2869DictionaryDeclaresNoAttributeForAnUnofferedService`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2869_unoffered_service_attributes_test.go#L129) |
| `RFC2865-1.1-2` | A NAS MUST treat a RADIUS access-accept authorizing an unavailable service as an access-reject instead, and MUST treat unknown or unsupported Service-Types the same way (§1.1, restated at §5.6) | MUST | 1.1 - Specification of Requirements | **positive:** `unit/verify` [`TestAdminAccessAcceptWithOfferedServiceTypeIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L61). **positive:** `unit/verify` [`TestRFC2865SubscriberServiceTypeAuthorization`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_nas_obligations_test.go#L208). **positive:** `unit/verify` [`TestRFC2865UnsupportedServiceTypeIsRejection`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L224). **negative:** `unit/verify` [`TestAdminAccessAcceptWithUnofferedServiceTypeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L82). **negative:** `unit/verify` [`TestRFC2865SubscriberServiceTypeAuthorization`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_nas_obligations_test.go#L215). **negative:** `unit/verify` [`TestRFC2865UnsupportedServiceTypeIsRejection`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L231) |
| `RFC2865-3-6` | Octets outside the range of the Length field MUST be treated as padding and ignored on reception (§3) | MUST | 3 - Packet Format | **positive:** `unit/verify` [`TestDecodeIgnoresOctetsOutsideTheLengthField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L104). **positive:** `unit/verify` [`TestRFC2866LengthPaddingIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L117). **negative:** `unit/verify` [`TestDecodeIgnoresAnAttributeHiddenInThePadding`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L120). **negative:** `unit/verify` [`TestRFC2866LengthPaddingBoundaryIsTheLengthField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L137) |
| `RFC2865-3-8` | The secret MUST NOT be empty (length 0) since this would allow packets to be trivially forged (§3) | MUST NOT | 3 - Packet Format | **positive:** `unit/verify` [`TestExchangeAcceptsANonEmptySharedSecret`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L159). **positive:** `unit/verify` [`TestRFC2865EmptySharedSecretBuildsNoClient`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L158). **negative:** `unit/verify` [`TestExchangeRefusesAnEmptySharedSecret`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L179). **negative:** `unit/verify` [`TestRFC2865EmptySharedSecretBuildsNoClient`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L163) |
| `RFC2865-4.1-1` | An implementation wishing to authenticate a user MUST transmit a RADIUS packet with the Code field set to 1 (Access-Request) (§4.1) | MUST | 4.1 - Access-Request | **positive:** `unit/verify` [`TestAdminAuthenticationTransmitsAccessRequest`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L198). **positive:** `unit/verify` [`TestRADIUSAuthNASIPAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/handler_test.go#L405). **negative:** `unit/verify` [`TestRFC2865AccountingRequestDoesNotCarryTheAccessRequestCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_access_request_shape_test.go#L96) |
| `RFC2865-4.1-6` | The Request Authenticator value MUST be changed each time a new Identifier is used (§4.1) | MUST | 4.1 - Access-Request | **positive:** `unit/verify` [`TestFailoverChangesTheRequestAuthenticatorWithTheIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L308). **positive:** `unit/verify` [`TestRFC2865FailoverRegeneratesRequestAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L107). **negative:** `unit/verify` [`TestRFC2865FailoverRegeneratesRequestAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L131). **negative:** `unit/verify` [`TestRetransmitToTheSameServerKeepsItsRequestAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L345) |
| `RFC2865-4.1-2` | An Access-Request MUST contain either a NAS-IP-Address attribute or a NAS-Identifier attribute (or both) (§4.1, restated at §5.44 Note 2) | MUST | 4.1 - Access-Request | **positive:** `unit/verify` [`TestAdminAccessRequestIdentifiesTheNAS`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L216). **positive:** `unit/verify` [`TestRADIUSAuthNASIPAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/handler_test.go#L407). **negative:** `unit/verify` [`TestRFC2865AccessRequestNamesTheNASWithNoIdentityConfigured`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_access_request_shape_test.go#L49) |
| `RFC2865-4.1-3` | An Access-Request MUST contain either a User-Password or a CHAP-Password or a State (§4.1, restated at §5.44 Note 1) | MUST | 4.1 - Access-Request | **positive:** `unit/verify` [`TestAdminAccessRequestCarriesExactlyOneCredential`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L237). **positive:** `unit/verify` [`TestRFC2865SubscriberAccessRequestCarriesCredential`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_nas_obligations_test.go#L101). **negative:** `unit/verify` [`TestRFC2865SubscriberAccessRequestCarriesCredential`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_nas_obligations_test.go#L128) |
| `RFC2865-4.4-1` | A NAS that does not support challenge/response MUST treat an Access-Challenge as though it had received an Access-Reject instead (§4.4) | MUST | 4.4 - Access-Challenge | **positive:** `unit/verify` [`TestAccessChallengeIsTreatedAsAccessReject`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L371). **positive:** `unit/verify` [`TestRFC2865AccessChallengeIsRejection`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L183). **negative:** `unit/verify` [`TestAccessChallengeDoesNotFallThroughToTheNextBackend`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L396). **negative:** `unit/verify` [`TestRFC2865AccessChallengeIsRejection`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L190) |
| `RFC2865-5-4` | A RADIUS server or client MUST NOT have any dependencies on the order of attributes of different types (§5) | MUST NOT | 5 - Attributes | **positive:** `unit/verify` [`TestAttributeLookupIgnoresTheOrderOfDifferentTypes`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L417). **positive:** `unit/verify` [`TestRFC2865AccessAcceptExtractionIsOrderIndependent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_attribute_order_test.go#L66). **negative:** `unit/verify` [`TestRFC2865AttributeOrderIsObservableByPosition`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_attribute_order_test.go#L101) |
| `RFC2865-5-8` | Text or String of length zero (0) MUST NOT be sent; omit the entire attribute instead (§5) | MUST NOT | 5 - Attributes | **positive:** `unit/verify` [`TestRFC2865SubscriberZeroLengthUserNameOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_nas_obligations_test.go#L159). **positive:** `unit/verify` [`TestRFC2865ZeroLengthTextIsOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L264). **positive:** `unit/verify` [`TestZeroLengthAttributeIsOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L511). **negative:** `unit/verify` [`TestOneOctetAttributeIsNotOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L528). **negative:** `unit/verify` [`TestRFC2865SubscriberZeroLengthUserNameOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_nas_obligations_test.go#L171). **negative:** `unit/verify` [`TestRFC2865ZeroLengthTextIsOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L272) |
| `RFC2865-3-7` | A packet shorter than its Length field indicates MUST be silently discarded (§3) | MUST | 3 - Packet Format | **positive:** `unit/verify` [`TestDecodeAcceptsAPacketAsLongAsItsLengthField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L136). **negative:** `unit/verify` [`TestDecodeRefusesAPacketShorterThanItsLengthField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L147) |
| `RFC2865-4.1-4` | An Access-Request MUST NOT contain both a User-Password and a CHAP-Password (§4.1, §5.44) | MUST NOT | 4.1 - Access-Request | **positive:** `unit/verify` [`TestAdminAccessRequestCarriesExactlyOneCredential`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L239). **negative:** no negative test. **{single-polarity}:** the credential method is a switch with one arm per method, so no builder adds both and there is no both-credentials path to drive a negative |
| `RFC2865-4.1-5` | The Identifier field MUST be changed whenever the content of the Attributes field changes, and whenever a valid reply has been received for a previous request (§4.1, §2.5) | MUST | 4.1 - Access-Request | **positive:** `unit/verify` [`TestSuccessiveRequestsUseDifferentIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L258). **negative:** `unit/verify` [`TestRetransmitToTheSameServerKeepsItsIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L281) |
| `RFC2865-5-5` | A RADIUS server or client MUST NOT require attributes of the same type to be contiguous (§5) | MUST NOT | 5 - Attributes | **positive:** `unit/verify` [`TestAttributeLookupDoesNotRequireContiguity`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L434). **negative:** no negative test. **{single-polarity}:** FindAllAttr walks the whole attribute list, so no contiguity-requiring path exists to drive a negative |
| `RFC2865-5-6` | An Attribute received in an Access-Accept, Access-Reject or Access-Challenge with an invalid length MUST cause the packet to be treated as an Access-Reject or else silently discarded (§5) | MUST | 5 - Attributes | **positive:** `unit/verify` [`TestResponseWithValidAttributeLengthsIsDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L449). **negative:** `unit/verify` [`TestResponseWithAnInvalidAttributeLengthIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L462) |
| `RFC2865-5-7` | Servers and clients MUST be able to deal with embedded nulls (§5) | MUST | 5 - Attributes | **positive:** `unit/verify` [`TestAttributeValueWithAnEmbeddedNullRoundTrips`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L479). **negative:** `unit/verify` [`TestAnEmbeddedNullDoesNotEndTheAttributeWalk`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L494) |
| `RFC2865-5.11-1` | A human-readable or opaque carrier attribute (Filter-Id §5.11, Reply-Message §5.18, Framed-Route §5.22, Vendor-Specific §5.26, Proxy-State §5.33) MUST NOT affect operation of the RADIUS protocol | MUST NOT | 5.11 - Filter-Id | **positive:** `unit/verify` [`TestCarrierAttributesDoNotAffectAnAccessAccept`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L546). **negative:** `unit/verify` [`TestCarrierAttributesDoNotAffectAnAccessReject`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L563) |
| `RFC2865-5.25-1` | The client MUST NOT interpret the State (§5.24) or Class (§5.25) attribute locally | MUST NOT | 5.25 - Class | **positive:** `unit/verify` [`TestRadiusClassIsNotInterpretedLocally`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_test.go#L303). **positive:** `unit/verify` [`TestStateIsNotInterpretedLocally`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L583). **negative:** `unit/verify` [`TestExtractRadiusConfigProfileAttrNeverClass`](https://github.com/ze-software/ze/blob/main/internal/component/radius/config_test.go#L101) |
| `RFC2865-2.5-2` | NAS SHOULD use exponential backoff between retransmits (§2.5) | SHOULD | 2.5 - Retransmission Hints | **positive:** no positive test. **negative:** no negative test |
| `RFC2865-x-1` | Use constant-time comparison for authenticator verification (Implementation Constraints) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC2865-5-3` | Server MAY include Reply-Message attribute in Access-Reject (§5) | MAY | 5 - Attributes | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 2865 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2865-3-1`](#rfc2865-3-1)

Packet Length MUST be between 20 and 4096 bytes (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeBadLength`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L193) | unit/verify | unproven |
| negative | [`TestDecodeTooLong`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L180) | unit/verify | unproven |
| negative | [`TestDecodeTooShort`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L171) | unit/verify | unproven |
| positive | [`TestPacketRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L141) | unit/verify | unproven |

### [`RFC2865-3-2`](#rfc2865-3-2)

Request Authenticator MUST be 16 cryptographically random octets (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2865RequestAuthenticatorRandom`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_test.go#L40) | unit/verify | unproven |

### [`RFC2865-3-3`](#rfc2865-3-3)

Response Authenticator MUST be MD5(Code+ID+Length+RequestAuth+Attributes+Secret) (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestResponseAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L214) | unit/verify | unproven |
| negative | [`TestVerifyResponseAuth`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L246) | unit/verify | unproven |
| negative | [`TestRFC2865ResponseAuthenticatorCoversEveryNamedField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_response_auth_test.go#L99) | unit/verify | unproven |
| positive | [`TestResponseAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L206) | unit/verify | unproven |
| positive | [`TestVerifyResponseAuth`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L240) | unit/verify | unproven |
| positive | [`TestRFC2865ResponseAuthenticatorMatchesTheFormula`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_response_auth_test.go#L58) | unit/verify | unproven |

### [`RFC2865-3-4`](#rfc2865-3-4)

NAS MUST verify Response Authenticator before trusting a response (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestClientAuthenticatorVerify`](https://github.com/ze-software/ze/blob/main/internal/component/radius/client_test.go#L388) | unit/verify | unproven |
| positive | [`TestClientAuthenticatorVerify`](https://github.com/ze-software/ze/blob/main/internal/component/radius/client_test.go#L383) | unit/verify | unproven |

### [`RFC2865-2.5-1`](#rfc2865-2.5-1)

A retransmitted request MUST use the same Identifier and Request Authenticator (§2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestClientRetransmit`](https://github.com/ze-software/ze/blob/main/internal/component/radius/client_test.go#L194) | unit/verify | unproven |

### [`RFC2865-5-1`](#rfc2865-5-1)

The User-Name attribute MUST be sent in Access-Request packets if available (§5, sentence at §5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2865AccessRequestOmitsAnUnavailableUserName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_access_request_shape_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestRFC2865SubscriberAccessRequestUserName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_test.go#L31) | unit/verify | unproven |
| positive | [`TestRFC2865AccessRequestUserName`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_test.go#L219) | unit/verify | unproven |

### [`RFC2865-5.2-1`](#rfc2865-5.2-1)

User-Password encoding MUST use MD5-based XOR chain: c[0] = p[0] XOR MD5(S+RA), c[i] = p[i] XOR MD5(S+c[i-1]) (§5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2865UserPasswordDependsOnSecret`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_test.go#L156) | unit/verify | unproven |
| positive | [`TestEncodeUserPassword`](https://github.com/ze-software/ze/blob/main/internal/component/radius/attr_test.go#L35) | unit/verify | unproven |
| positive | [`TestEncodeUserPasswordMultiBlock`](https://github.com/ze-software/ze/blob/main/internal/component/radius/attr_test.go#L97) | unit/verify | unproven |

### [`RFC2865-5.2-2`](#rfc2865-5.2-2)

User-Password MUST be padded to a multiple of 16 octets (max 128) (§5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEncodeUserPassword`](https://github.com/ze-software/ze/blob/main/internal/component/radius/attr_test.go#L17) | unit/verify | unproven |
| positive | [`TestEncodeUserPasswordEmpty`](https://github.com/ze-software/ze/blob/main/internal/component/radius/attr_test.go#L52) | unit/verify | unproven |
| positive | [`TestEncodeUserPasswordMultiBlock`](https://github.com/ze-software/ze/blob/main/internal/component/radius/attr_test.go#L75) | unit/verify | unproven |
| positive | [`TestRFC2865UserPasswordClamp`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_test.go#L175) | unit/verify | unproven |

### [`RFC2865-5-2`](#rfc2865-5-2)

Attribute length MUST NOT exceed 255 bytes (Type + Length + Value) (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2865AttributeLengthBound`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_test.go#L94) | unit/verify | unproven |
| positive | [`TestRFC2865AttributeLengthBound`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_test.go#L78) | unit/verify | unproven |

### [`RFC2865-3-5`](#rfc2865-3-5)

Only accept responses from the server address the request was sent to (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2865ResponseSourceAddress`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_test.go#L130) | unit/verify | unproven |
| positive | [`TestClientExchangeAccept`](https://github.com/ze-software/ze/blob/main/internal/component/radius/client_test.go#L105) | unit/verify | unproven |
| positive | [`TestRFC2865ResponseSourceAddress`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_test.go#L139) | unit/verify | unproven |

### [`RFC2865-1.1-1`](#rfc2865-1.1-1)

A NAS that does not implement a given service MUST NOT implement the RADIUS attributes for that service (§1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2869DictionaryDeclaresNoAttributeForAnUnofferedService`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2869_unoffered_service_attributes_test.go#L129) | unit/verify | unproven |
| positive | [`TestAdminAccessRequestCarriesNoUnofferedServiceAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestRFC2869DictionaryCoversTheServicesZeOffers`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2869_unoffered_service_attributes_test.go#L99) | unit/verify | unproven |

### [`RFC2865-1.1-2`](#rfc2865-1.1-2)

A NAS MUST treat a RADIUS access-accept authorizing an unavailable service as an access-reject instead, and MUST treat unknown or unsupported Service-Types the same way (§1.1, restated at §5.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2865SubscriberServiceTypeAuthorization`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_nas_obligations_test.go#L215) | unit/verify | unproven |
| negative | [`TestRFC2865UnsupportedServiceTypeIsRejection`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L231) | unit/verify | unproven |
| negative | [`TestAdminAccessAcceptWithUnofferedServiceTypeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L82) | unit/verify | revert, verified |
| positive | [`TestRFC2865SubscriberServiceTypeAuthorization`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_nas_obligations_test.go#L208) | unit/verify | unproven |
| positive | [`TestRFC2865UnsupportedServiceTypeIsRejection`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L224) | unit/verify | unproven |
| positive | [`TestAdminAccessAcceptWithOfferedServiceTypeIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L61) | unit/verify | revert, verified |

### [`RFC2865-3-6`](#rfc2865-3-6)

Octets outside the range of the Length field MUST be treated as padding and ignored on reception (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeIgnoresAnAttributeHiddenInThePadding`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L120) | unit/verify | revert, verified |
| negative | [`TestRFC2866LengthPaddingBoundaryIsTheLengthField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L137) | unit/verify | unproven |
| positive | [`TestDecodeIgnoresOctetsOutsideTheLengthField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestRFC2866LengthPaddingIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L117) | unit/verify | unproven |

### [`RFC2865-3-8`](#rfc2865-3-8)

The secret MUST NOT be empty (length 0) since this would allow packets to be trivially forged (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2865EmptySharedSecretBuildsNoClient`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L163) | unit/verify | unproven |
| negative | [`TestExchangeRefusesAnEmptySharedSecret`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L179) | unit/verify | revert, verified |
| positive | [`TestRFC2865EmptySharedSecretBuildsNoClient`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L158) | unit/verify | unproven |
| positive | [`TestExchangeAcceptsANonEmptySharedSecret`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L159) | unit/verify | revert, verified |

### [`RFC2865-4.1-1`](#rfc2865-4.1-1)

An implementation wishing to authenticate a user MUST transmit a RADIUS packet with the Code field set to 1 (Access-Request) (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2865AccountingRequestDoesNotCarryTheAccessRequestCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_access_request_shape_test.go#L96) | unit/verify | revert, verified |
| positive | [`TestRADIUSAuthNASIPAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/handler_test.go#L405) | unit/verify | revert, verified |
| positive | [`TestAdminAuthenticationTransmitsAccessRequest`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L198) | unit/verify | revert, verified |

### [`RFC2865-4.1-6`](#rfc2865-4.1-6)

The Request Authenticator value MUST be changed each time a new Identifier is used (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2865FailoverRegeneratesRequestAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L131) | unit/verify | unproven |
| negative | [`TestRetransmitToTheSameServerKeepsItsRequestAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L345) | unit/verify | revert, verified |
| positive | [`TestRFC2865FailoverRegeneratesRequestAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L107) | unit/verify | unproven |
| positive | [`TestFailoverChangesTheRequestAuthenticatorWithTheIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L308) | unit/verify | revert, verified |

### [`RFC2865-4.1-2`](#rfc2865-4.1-2)

An Access-Request MUST contain either a NAS-IP-Address attribute or a NAS-Identifier attribute (or both) (§4.1, restated at §5.44 Note 2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2865AccessRequestNamesTheNASWithNoIdentityConfigured`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_access_request_shape_test.go#L49) | unit/verify | revert, verified |
| positive | [`TestRADIUSAuthNASIPAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/handler_test.go#L407) | unit/verify | revert, verified |
| positive | [`TestAdminAccessRequestIdentifiesTheNAS`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L216) | unit/verify | revert, verified |

### [`RFC2865-4.1-3`](#rfc2865-4.1-3)

An Access-Request MUST contain either a User-Password or a CHAP-Password or a State (§4.1, restated at §5.44 Note 1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2865SubscriberAccessRequestCarriesCredential`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_nas_obligations_test.go#L128) | unit/verify | unproven |
| positive | [`TestRFC2865SubscriberAccessRequestCarriesCredential`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_nas_obligations_test.go#L101) | unit/verify | unproven |
| positive | [`TestAdminAccessRequestCarriesExactlyOneCredential`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L237) | unit/verify | revert, verified |

### [`RFC2865-4.4-1`](#rfc2865-4.4-1)

A NAS that does not support challenge/response MUST treat an Access-Challenge as though it had received an Access-Reject instead (§4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2865AccessChallengeIsRejection`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L190) | unit/verify | unproven |
| negative | [`TestAccessChallengeDoesNotFallThroughToTheNextBackend`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L396) | unit/verify | revert, verified |
| positive | [`TestRFC2865AccessChallengeIsRejection`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L183) | unit/verify | unproven |
| positive | [`TestAccessChallengeIsTreatedAsAccessReject`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L371) | unit/verify | revert, verified |

### [`RFC2865-5-4`](#rfc2865-5-4)

A RADIUS server or client MUST NOT have any dependencies on the order of attributes of different types (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2865AttributeOrderIsObservableByPosition`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_attribute_order_test.go#L101) | unit/verify | revert, verified |
| positive | [`TestRFC2865AccessAcceptExtractionIsOrderIndependent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_attribute_order_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestAttributeLookupIgnoresTheOrderOfDifferentTypes`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L417) | unit/verify | revert, verified |

### [`RFC2865-5-8`](#rfc2865-5-8)

Text or String of length zero (0) MUST NOT be sent; omit the entire attribute instead (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2865SubscriberZeroLengthUserNameOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_nas_obligations_test.go#L171) | unit/verify | unproven |
| negative | [`TestRFC2865ZeroLengthTextIsOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L272) | unit/verify | unproven |
| negative | [`TestOneOctetAttributeIsNotOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L528) | unit/verify | revert, verified |
| positive | [`TestRFC2865SubscriberZeroLengthUserNameOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2865_nas_obligations_test.go#L159) | unit/verify | unproven |
| positive | [`TestRFC2865ZeroLengthTextIsOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_nas_obligations_test.go#L264) | unit/verify | unproven |
| positive | [`TestZeroLengthAttributeIsOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L511) | unit/verify | revert, verified |

### [`RFC2865-3-7`](#rfc2865-3-7)

A packet shorter than its Length field indicates MUST be silently discarded (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeRefusesAPacketShorterThanItsLengthField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L147) | unit/verify | revert, verified |
| positive | [`TestDecodeAcceptsAPacketAsLongAsItsLengthField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L136) | unit/verify | revert, verified |

### [`RFC2865-4.1-4`](#rfc2865-4.1-4)

An Access-Request MUST NOT contain both a User-Password and a CHAP-Password (§4.1, §5.44)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestAdminAccessRequestCarriesExactlyOneCredential`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L239) | unit/verify | revert, verified |

### [`RFC2865-4.1-5`](#rfc2865-4.1-5)

The Identifier field MUST be changed whenever the content of the Attributes field changes, and whenever a valid reply has been received for a previous request (§4.1, §2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRetransmitToTheSameServerKeepsItsIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L281) | unit/verify | revert, verified |
| positive | [`TestSuccessiveRequestsUseDifferentIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L258) | unit/verify | revert, verified |

### [`RFC2865-5-5`](#rfc2865-5-5)

A RADIUS server or client MUST NOT require attributes of the same type to be contiguous (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestAttributeLookupDoesNotRequireContiguity`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L434) | unit/verify | revert, verified |

### [`RFC2865-5-6`](#rfc2865-5-6)

An Attribute received in an Access-Accept, Access-Reject or Access-Challenge with an invalid length MUST cause the packet to be treated as an Access-Reject or else silently discarded (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestResponseWithAnInvalidAttributeLengthIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L462) | unit/verify | revert, verified |
| positive | [`TestResponseWithValidAttributeLengthsIsDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L449) | unit/verify | revert, verified |

### [`RFC2865-5-7`](#rfc2865-5-7)

Servers and clients MUST be able to deal with embedded nulls (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAnEmbeddedNullDoesNotEndTheAttributeWalk`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L494) | unit/verify | revert, verified |
| positive | [`TestAttributeValueWithAnEmbeddedNullRoundTrips`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L479) | unit/verify | revert, verified |

### [`RFC2865-5.11-1`](#rfc2865-5.11-1)

A human-readable or opaque carrier attribute (Filter-Id §5.11, Reply-Message §5.18, Framed-Route §5.22, Vendor-Specific §5.26, Proxy-State §5.33) MUST NOT affect operation of the RADIUS protocol

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCarrierAttributesDoNotAffectAnAccessReject`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L563) | unit/verify | revert, verified |
| positive | [`TestCarrierAttributesDoNotAffectAnAccessAccept`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L546) | unit/verify | revert, verified |

### [`RFC2865-5.25-1`](#rfc2865-5.25-1)

The client MUST NOT interpret the State (§5.24) or Class (§5.25) attribute locally

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtractRadiusConfigProfileAttrNeverClass`](https://github.com/ze-software/ze/blob/main/internal/component/radius/config_test.go#L101) | unit/verify | unproven |
| positive | [`TestRadiusClassIsNotInterpretedLocally`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_test.go#L303) | unit/verify | unproven |
| positive | [`TestStateIsNotInterpretedLocally`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_walk_test.go#L583) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-implement walk agent, spec-rfcgate-6-supported-extraction-signoff phase 2 (Tier 1, rfc2865) |
| Signed off | 2026-08-30 |
| Register | rfc2119 |
| Source | rfc/full/rfc2865.txt |
| Source fingerprint | 5082eacae1b57b82 |
| Record | rfc/extraction/rfc2865.json |
| Mapped sentences | 22 |
| Declined as scope | 52 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, Copyright Notice, Abstract and Table of Contents. The Abstract restates section 1 and states no obligation. |
| `1` | not stated | 1 | walked | Introduction: what RADIUS is, its client/server model, its network security model and its extensibility. It states no obligation of its own. Its one derived site is the Section 5.44 table legend, which the section parser attached here because the legend line begins with '1'. |
| `1.1` | Specification of Requirements | 3 | walked | Specification of Requirements. It carries the RFC 2119 key-words paragraph the site scan correctly refuses to count, the definition of unconditional and conditional compliance, and three NAS obligations about services a NAS does not implement (sites 1.1:1 to 1.1:3). |
| `1.2` | Terminology: service, session and 'silently discard' | 0 | walked | Terminology: service, session and 'silently discard'. The two keywords in the 'silently discard' entry are SHOULDs about logging and counting a discarded packet, so the scan derives no site. |
| `2` | Operation | 2 | walked | Operation. It narrates one authentication exchange end to end. Its two sites are both server obligations. The client-side sentences here are indicative or MAY-level and are stated normatively in sections 3, 4.1 and 4.4. |
| `2.1` | Challenge/Response | 0 | walked | Challenge/Response. Descriptive: what a challenge is, what the user does with it, and a worked example. It states no obligation, and the NAS obligation for a challenge is in section 4.4. |
| `2.2` | Interoperation with PAP and CHAP | 2 | walked | Interoperation with PAP and CHAP. It describes how a NAS maps PAP and CHAP credentials onto User-Password and CHAP-Password, in indicative prose, and then states two server obligations (sites 2.2:1 and 2.2:2). |
| `2.3` | Proxy | 10 | walked | Proxy. Ten sites, every one binding a forwarding server or a remote server. Ze forwards no RADIUS request and answers none, so the whole section is excluded by role. |
| `2.4` | Why UDP? Design rationale for the transport choice | 0 | walked | Why UDP? Design rationale for the transport choice. It uses 'must' in lowercase throughout to describe consequences, and states no obligation. |
| `2.5` | Retransmission Hints | 2 | walked | Retransmission Hints. Its two sites are the same-server retransmission rule (site 2.5:1) and the changed-attributes rule (site 2.5:2), which Section 4.1 states in full. The exponential-backoff row RFC2865-2.5-2 is a ze implementation constraint at SHOULD level, not a sentence of this section, and a SHOULD row never gates. |
| `2.6` | Keep-Alives Considered Harmful | 0 | walked | Keep-Alives Considered Harmful. Advice against probing a server, phrased as 'strongly discouraged'. No capitalised keyword and no obligation. |
| `3` | Packet Format | 5 | walked | Packet Format. The header layout, the Code, Identifier, Length and Authenticator fields, the Request and Response Authenticator definitions, and the Administrative Note on the shared secret. Five sites: three bind any receiver or both ends and are mapped, two bind a server or a proxy. |
| `4` | Packet Types | 0 | walked | Packet Types. One sentence saying the Code field decides the type. The four types are 4.1 to 4.4. |
| `4.1` | Access-Request | 8 | walked | Access-Request. Eight sites. Six bind the sender and are mapped or duplicated onto Section 2.5; one binds the server that must reply; one is the retransmission half of the Identifier rule. |
| `4.2` | Access-Accept | 2 | walked | Access-Accept. Two sites: the server's obligation to transmit it, and the receiver's obligation to check the Response Authenticator against the pending request. |
| `4.3` | Access-Reject | 1 | walked | Access-Reject. One site, binding the server that transmits it. The Reply-Message sentence is MAY-level and is the summary's RFC2865-5-3. |
| `4.4` | Access-Challenge | 4 | walked | Access-Challenge. Four sites: two bind the server, and two state what a NAS without challenge/response support owes, which is RFC2865-4.4-1. |
| `5` | Attributes | 7 | walked | Attributes. The attribute format, the Type, Length and Value fields, and the five data types. Seven sites: five bind a client and are mapped, one binds a proxy, one repeats the zero-length rule for String. |
| `5.1` | User-Name | 1 | walked | User-Name. One site: the attribute MUST be sent in an Access-Request if available. |
| `5.2` | User-Password | 0 | walked | User-Password. The hiding algorithm, stated entirely in indicative prose and as a set of equations, so the scan derives no site. The two obligations the summary declares from it are listed below. |
| `5.3` | CHAP-Password | 0 | walked | CHAP-Password. Format and the rule that the challenge is in CHAP-Challenge if present and the Request Authenticator otherwise. Indicative prose, no site. |
| `5.4` | NAS-IP-Address | 3 | walked | NAS-IP-Address. Three sites: the Section 4.1 identification rule restated, and the two shared-secret-selection sentences that bind the server. |
| `5.5` | NAS-Port | 0 | walked | NAS-Port. Attribute format and a note on 16-bit port values. No obligation. |
| `5.6` | Service-Type | 1 | walked | Service-Type. One site: a NAS MUST treat an unknown or unsupported Service-Type as an Access-Reject. The rest is the value table. |
| `5.7` | Framed-Protocol | 0 | walked | Framed-Protocol. Value table for PPP, SLIP, ARAP, Gandalf, Xylogics and X.75. No obligation. |
| `5.8` | Framed-IP-Address | 0 | walked | Framed-IP-Address. Format and the two special values 0xFFFFFFFF and 0xFFFFFFFE. No obligation. |
| `5.9` | Framed-IP-Netmask | 0 | walked | Framed-IP-Netmask. Format. No obligation. |
| `5.10` | Framed-Routing | 0 | walked | Framed-Routing. Value table. No obligation. |
| `5.11` | Filter-Id | 1 | walked | Filter-Id. One site: the attribute is human readable and MUST NOT affect operation of the protocol. |
| `5.12` | Framed-MTU | 0 | walked | Framed-MTU. Format and range. No obligation. |
| `5.13` | Framed-Compression | 0 | walked | Framed-Compression. Value table. No obligation. |
| `5.14` | Login-IP-Host | 0 | walked | Login-IP-Host. Format and the two special values. No obligation. |
| `5.15` | Login-Service | 0 | walked | Login-Service. Value table. No obligation. |
| `5.16` | Login-TCP-Port | 0 | walked | Login-TCP-Port. Format. No obligation. |
| `5.17` | Unassigned attribute type 17 | 0 | walked | Unassigned attribute type 17. A heading and a note. No obligation. |
| `5.18` | Reply-Message | 2 | walked | Reply-Message. Two sites: the display-order rule for several messages, and the human-readable sentence the other carrier attributes repeat. |
| `5.19` | Callback-Number | 0 | walked | Callback-Number. Format. No obligation. |
| `5.20` | Callback-Id | 0 | walked | Callback-Id. Format and a note that the id is NAS specific. No obligation. |
| `5.21` | Unassigned attribute type 21 | 0 | walked | Unassigned attribute type 21. A heading and a note. No obligation. |
| `5.22` | Framed-Route | 1 | walked | Framed-Route. One site: the human-readable sentence. The route text format is stated in indicative prose. |
| `5.23` | Framed-IPX-Network | 0 | walked | Framed-IPX-Network. Format and the special value 0xFFFFFFFE. No obligation. |
| `5.24` | State | 3 | walked | State. Three sites: the challenge-reply carry rule, the Termination-Action carry rule, and the prohibition on interpreting the attribute locally. |
| `5.25` | Class | 1 | walked | Class. One site: the client MUST NOT interpret the attribute locally. The accounting-echo sentence beside it is a SHOULD. |
| `5.26` | Vendor-Specific | 2 | walked | Vendor-Specific. Two sites: the attribute MUST NOT affect protocol operation, and a server not equipped to interpret vendor information MUST ignore it. |
| `5.27` | Session-Timeout | 0 | walked | Session-Timeout. Format and meaning. No obligation. |
| `5.28` | Idle-Timeout | 0 | walked | Idle-Timeout. Format and meaning. No obligation. |
| `5.29` | Termination-Action | 0 | walked | Termination-Action. Value table. No obligation. |
| `5.30` | Called-Station-Id | 0 | walked | Called-Station-Id. Format and a note on dialed-number identification. No obligation. |
| `5.31` | Calling-Station-Id | 0 | walked | Calling-Station-Id. Format and a note on automatic number identification. No obligation. |
| `5.32` | NAS-Identifier | 3 | walked | NAS-Identifier. Three sites, the same three sentences Section 5.4 carries for NAS-IP-Address. |
| `5.33` | Proxy-State | 4 | walked | Proxy-State. Four sites, every one binding a proxy server or a server that adds a Proxy-State. |
| `5.34` | Login-LAT-Service | 0 | walked | Login-LAT-Service. Format and a note on LAT string handling. No obligation, and ze offers no LAT service. |
| `5.35` | Login-LAT-Node | 0 | walked | Login-LAT-Node. Format. No obligation, and ze offers no LAT service. |
| `5.36` | Login-LAT-Group | 0 | walked | Login-LAT-Group. Format of the 32-octet group code bitmap. No obligation, and ze offers no LAT service. |
| `5.37` | Framed-AppleTalk-Link | 0 | walked | Framed-AppleTalk-Link. Format. No obligation, and ze offers no AppleTalk service. |
| `5.38` | Framed-AppleTalk-Network | 0 | walked | Framed-AppleTalk-Network. Format. No obligation, and ze offers no AppleTalk service. |
| `5.39` | Framed-AppleTalk-Zone | 0 | walked | Framed-AppleTalk-Zone. Format. No obligation, and ze offers no AppleTalk service. |
| `5.40` | CHAP-Challenge | 0 | walked | CHAP-Challenge. Format and the rule that a 16-octet challenge may travel in the Request Authenticator instead. Indicative prose, no site. |
| `5.41` | NAS-Port-Type | 0 | walked | NAS-Port-Type. Value table. No obligation. |
| `5.42` | Port-Limit | 0 | walked | Port-Limit. Format and meaning. No obligation. |
| `5.43` | Login-LAT-Port | 0 | walked | Login-LAT-Port. Format. No obligation, and ze offers no LAT service. |
| `5.44` | Table of Attributes | 3 | walked | Table of Attributes. Three sites, all in Note 1 and Note 2, restating Section 4.1's credential and identification rules. The table's own cells and its legend are covered by the two legend sites the section parser split off into sections '1' and '0'. |
| `0` | not stated | 1 | walked | The trailing legend block of the Section 5.44 Table of Attributes, which the section parser split into a section of its own because its first line begins with '0'. Its one site is the '0' legend entry. |
| `6` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. It names the registries and the assignment process for RADIUS packet type codes, attribute types and attribute values. Binds IANA, not a speaker. |
| `6.1` | not stated | 0 | skipped (iana) | Definition of Terms for the IANA section: Private Use, Specification Required, Designated Expert and so on. |
| `6.2` | Recommended Registration Policies | 0 | skipped (iana) | Recommended Registration Policies. It tells IANA which policy governs each RADIUS namespace. |
| `7` | Examples | 0 | walked | Examples. Three worked exchanges shown as attribute dumps. Non-normative illustration of sections 4 and 5, and it states no obligation. |
| `7.1` | Example: user Telnet to a specified host | 0 | walked | Example: user Telnet to a specified host. An attribute dump of one Access-Request and its Access-Accept. |
| `7.2` | Example: framed user authenticating with CHAP | 0 | walked | Example: framed user authenticating with CHAP. An attribute dump of one exchange. |
| `7.3` | Example: user with a challenge-response card | 0 | walked | Example: user with a challenge-response card. An attribute dump of an Access-Request, an Access-Challenge and a second Access-Request. |
| `8` | Security Considerations | 0 | walked | Security Considerations. It discusses one authentication method per user name, secret storage, secret distribution and the known weakness of the Section 5.2 hiding mechanism. Its two keywords are SHOULDs, so the scan derives no site. |
| `9` | Change Log | 1 | walked | Change Log. It lists what changed from RFC 2138. Its one site quotes the Section 5 proxy ordering sentence while describing that change. |
| `10` | References, normative and informative | 0 | skipped (references) | References, normative and informative. |
| `11` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. |
| `12` | Chair's Address | 0 | skipped (front-matter) | Chair's Address. A contact block, the same boilerplate class as the title block at the head of the document. |
| `13` | Authors' Addresses | 0 | skipped (front-matter) | Authors' Addresses. A contact block. |
| `14` | Full Copyright Statement | 0 | skipped (front-matter) | Full Copyright Statement. The RFC boilerplate closing the document. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The legend of the Section 5.44 Table of Attributes, which the section parser attached to section 1 because the line begins with '1'. It defines what the table entry '1' means and binds no speaker: 'this attribute' names none. No column of that table carries '1' for any RFC 2865 attribute, so the case it defines is never exercised. | Exactly one instance of this attribute MUST be present in packet. |
| `1.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The ARAP example restating the sentence before it. RFC2865-1.1-1 carries the obligation and site 1.1:1 maps it. | For example, a NAS that is unable to offer ARAP service MUST NOT implement the RADIUS attributes for ARAP. |
| `2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS SERVER. Ze is a RADIUS CLIENT (a NAS): it sends Access-Requests and reads the reply. No file in the tree implements a RADIUS authentication server, and the only RADIUS listener ze runs is the RFC 5176 CoA/Disconnect port (internal/component/l2tp/plugins/authradius/coa.go), whose obligations belong to RFC 5176. | A request from a client for which the RADIUS server does not have a shared secret MUST be silently discarded. |
| `2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS SERVER. Ze is a RADIUS CLIENT (a NAS): it sends Access-Requests and reads the reply. No file in the tree implements a RADIUS authentication server, and the only RADIUS listener ze runs is the RFC 5176 CoA/Disconnect port (internal/component/l2tp/plugins/authradius/coa.go), whose obligations belong to RFC 5176. The obligation is to copy Proxy-State into the response it builds. | If any Proxy-State attributes were present in the Access-Request, they MUST be copied unmodified and in order into the response packet. |
| `2.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS SERVER. Ze is a RADIUS CLIENT (a NAS): it sends Access-Requests and reads the reply. No file in the tree implements a RADIUS authentication server, and the only RADIUS listener ze runs is the RFC 5176 CoA/Disconnect port (internal/component/l2tp/plugins/authradius/coa.go), whose obligations belong to RFC 5176. The obligation is to answer with an Access-Reject when it cannot perform the authentication. | If the RADIUS server is unable to perform the requested authentication it MUST return an Access-Reject. |
| `2.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS SERVER. Ze is a RADIUS CLIENT (a NAS): it sends Access-Requests and reads the reply. No file in the tree implements a RADIUS authentication server, and the only RADIUS listener ze runs is the RFC 5176 CoA/Disconnect port (internal/component/l2tp/plugins/authradius/coa.go), whose obligations belong to RFC 5176. The obligation turns on whether the server holds the password in cleartext, which is a server-side database question. | If the password is not available in cleartext to the RADIUS server then the server MUST send an Access-Reject to the client. |
| `2.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the FORWARDING SERVER of proxy RADIUS. Ze never forwards a RADIUS request: internal/component/radius/client.go sends its own Access-Requests and internal/component/l2tp/plugins/authradius does the same for subscribers. Ze adds no Proxy-State and reads none. | The forwarding server MUST treat any Proxy-State attributes already in the packet as opaque data. |
| `2.3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the FORWARDING SERVER of proxy RADIUS. Ze never forwards a RADIUS request: internal/component/radius/client.go sends its own Access-Requests and internal/component/l2tp/plugins/authradius does the same for subscribers. Ze adds no Proxy-State and reads none. | Its operation MUST NOT depend on the content of Proxy-State attributes added by previous servers. |
| `2.3:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the FORWARDING SERVER of proxy RADIUS. Ze never forwards a RADIUS request: internal/component/radius/client.go sends its own Access-Requests and internal/component/l2tp/plugins/authradius does the same for subscribers. Ze adds no Proxy-State and reads none. | If there are any Proxy-State attributes in the request received from the client, the forwarding server MUST include those Proxy-State attributes in its reply to the client. |
| `2.3:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the FORWARDING SERVER of proxy RADIUS. Ze never forwards a RADIUS request: internal/component/radius/client.go sends its own Access-Requests and internal/component/l2tp/plugins/authradius does the same for subscribers. Ze adds no Proxy-State and reads none. | If the forwarding server omits the Proxy-State attributes in the forwarded access-request, it MUST attach them to the response before sending it to the client. |
| `2.3:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the FORWARDING SERVER of proxy RADIUS. Ze never forwards a RADIUS request: internal/component/radius/client.go sends its own Access-Requests and internal/component/l2tp/plugins/authradius does the same for subscribers. Ze adds no Proxy-State and reads none. The Request-Authenticator-to-CHAP-Challenge copy is a forwarding act. | If a CHAP-Password attribute is present in the packet and no CHAP-Challenge attribute is present, the forwarding server MUST leave the Request- Authenticator untouched or copy it to a CHAP-Challenge attribute. |
| `2.3:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the FORWARDING SERVER of proxy RADIUS. Ze never forwards a RADIUS request: internal/component/radius/client.go sends its own Access-Requests and internal/component/l2tp/plugins/authradius does the same for subscribers. Ze adds no Proxy-State and reads none. It bounds the Proxy-State a forwarder may add. | (It MUST NOT add more than one.) If it adds a Proxy- State, the Proxy-State MUST appear after any other Proxy-States in the packet. |
| `2.3:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the FORWARDING SERVER of proxy RADIUS. Ze never forwards a RADIUS request: internal/component/radius/client.go sends its own Access-Requests and internal/component/l2tp/plugins/authradius does the same for subscribers. Ze adds no Proxy-State and reads none. | The forwarding server MUST NOT modify any other Proxy-States that were in the packet (it may choose not to forward them, but it MUST NOT change their contents). |
| `2.3:8` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the FORWARDING SERVER of proxy RADIUS. Ze never forwards a RADIUS request: internal/component/radius/client.go sends its own Access-Requests and internal/component/l2tp/plugins/authradius does the same for subscribers. Ze adds no Proxy-State and reads none. | The forwarding server MUST NOT change the order of any attributes of the same type, including Proxy-State. |
| `2.3:9` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the REMOTE SERVER at the end of a proxy chain, which builds the response. Ze implements no RADIUS server role at all, remote or forwarding. The producer that would act as it if ze did is ze's RADIUS code, which is a client only: `Exchange` and `SendToServers` (`internal/component/radius/client.go`) send a request and match the reply, and `Authenticate` (`internal/component/radius/authenticator.go`) consumes the response. Nothing serves, proxies or re-authenticates. | The remote server MUST copy all Proxy-State attributes (and only the Proxy-State attributes) in order from the access-request to the response packet, without modifying them. |
| `2.3:10` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the FORWARDING SERVER of proxy RADIUS. Ze never forwards a RADIUS request: internal/component/radius/client.go sends its own Access-Requests and internal/component/l2tp/plugins/authradius does the same for subscribers. Ze adds no Proxy-State and reads none. It bounds what local policy may rewrite in a packet passing through. | A forwarding server MUST not modify existing Proxy-State, State, or Class attributes present in the packet. |
| `2.5:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates in the retransmission-hints section what Section 4.1 states for the Identifier field and for the Request Authenticator. RFC2865-4.1-5 carries the Identifier half and site 4.1:6 maps it; RFC2865-4.1-6 carries the Request Authenticator half and site 4.1:8 maps that. | If any attributes have changed, you MUST use a new Request Authenticator and ID. |
| `3:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS SERVER. Ze is a RADIUS CLIENT (a NAS): it sends Access-Requests and reads the reply. No file in the tree implements a RADIUS authentication server, and the only RADIUS listener ze runs is the RFC 5176 CoA/Disconnect port (internal/component/l2tp/plugins/authradius/coa.go), whose obligations belong to RFC 5176. Selecting a shared secret from the source address of a received Access-Request is a server act. | A RADIUS server MUST use the source IP address of the RADIUS UDP packet to decide which shared secret to use, so that RADIUS requests can be proxied. |
| `3:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the FORWARDING SERVER of proxy RADIUS. Ze never forwards a RADIUS request: internal/component/radius/client.go sends its own Access-Requests and internal/component/l2tp/plugins/authradius does the same for subscribers. Ze adds no Proxy-State and reads none. The obligation is to strip the Proxy-State the proxy itself added. | When using a forwarding proxy, the proxy must be able to alter the packet as it passes through in each direction - when the proxy forwards the request, the proxy MAY add a Proxy-State Attribute, and when the proxy forwards a response, it MUST remove its Proxy- State Attribute if it added one. |
| `4.1:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS SERVER. Ze is a RADIUS CLIENT (a NAS): it sends Access-Requests and reads the reply. No file in the tree implements a RADIUS authentication server, and the only RADIUS listener ze runs is the RFC 5176 CoA/Disconnect port (internal/component/l2tp/plugins/authradius/coa.go), whose obligations belong to RFC 5176. The obligation is to transmit a reply on receipt of an Access-Request. | Upon receipt of an Access-Request from a valid client, an appropriate reply MUST be transmitted. |
| `4.1:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The retransmission half of the Identifier rule, which Section 2.5 states in full. RFC2865-2.5-1 carries it and site 2.5:1 maps it. | For retransmissions, the Identifier MUST remain unchanged. |
| `4.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS SERVER. Ze is a RADIUS CLIENT (a NAS): it sends Access-Requests and reads the reply. No file in the tree implements a RADIUS authentication server, and the only RADIUS listener ze runs is the RFC 5176 CoA/Disconnect port (internal/component/l2tp/plugins/authradius/coa.go), whose obligations belong to RFC 5176. The obligation is to transmit the Access-Accept. | If all Attribute values received in an Access-Request are acceptable then the RADIUS implementation MUST transmit a packet with the Code field set to 2 (Access-Accept). |
| `4.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS SERVER. Ze is a RADIUS CLIENT (a NAS): it sends Access-Requests and reads the reply. No file in the tree implements a RADIUS authentication server, and the only RADIUS listener ze runs is the RFC 5176 CoA/Disconnect port (internal/component/l2tp/plugins/authradius/coa.go), whose obligations belong to RFC 5176. The obligation is to transmit the Access-Reject. | If any value of the received Attributes is not acceptable, then the RADIUS server MUST transmit a packet with the Code field set to 3 (Access-Reject). |
| `4.4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS SERVER. Ze is a RADIUS CLIENT (a NAS): it sends Access-Requests and reads the reply. No file in the tree implements a RADIUS authentication server, and the only RADIUS listener ze runs is the RFC 5176 CoA/Disconnect port (internal/component/l2tp/plugins/authradius/coa.go), whose obligations belong to RFC 5176. The obligation is to transmit the Access-Challenge. | If the RADIUS server desires to send the user a challenge requiring a response, then the RADIUS server MUST respond to the Access-Request by transmitting a packet with the Code field set to 11 (Access-Challenge). |
| `4.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats for an Access-Challenge what Section 4.2 states for an Access-Accept. RFC2865-3-4 carries it and site 4.2:2 maps it. | Additionally, the Response Authenticator field MUST contain the correct response for the pending Access-Request. |
| `4.4:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The PAP-forwarding variant of the same sentence: a NAS that cannot forward the Reply-Message treats the challenge as a reject. RFC2865-4.4-1 carries it and site 4.4:3 maps it. | If the NAS cannot do so, it MUST treat the Access-Challenge as though it had received an Access-Reject instead. |
| `5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a PROXY: the sentence names the speaker, 'MUST be preserved by any proxies'. the FORWARDING SERVER of proxy RADIUS. Ze never forwards a RADIUS request: internal/component/radius/client.go sends its own Access-Requests and internal/component/l2tp/plugins/authradius does the same for subscribers. Ze adds no Proxy-State and reads none. | If multiple Attributes with the same Type are present, the order of Attributes with the same Type MUST be preserved by any proxies. |
| `5:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The String paragraph repeating the Text paragraph's sentence word for word. RFC2865-5-8 covers both and site 5:6 maps it. | Strings of length zero (0) MUST NOT be sent; omit the entire attribute instead. |
| `5.4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The NAS-IP-Address section restating the Section 4.1 rule. RFC2865-4.1-2 carries it and site 4.1:3 maps it. | Either NAS-IP- Address or NAS-Identifier MUST be present in an Access-Request packet. |
| `5.4:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS SERVER. Ze is a RADIUS CLIENT (a NAS): it sends Access-Requests and reads the reply. No file in the tree implements a RADIUS authentication server, and the only RADIUS listener ze runs is the RFC 5176 CoA/Disconnect port (internal/component/l2tp/plugins/authradius/coa.go), whose obligations belong to RFC 5176. Choosing which shared secret authenticates a received request is a server act, and this sentence forbids one input to that choice. | Note that NAS-IP-Address MUST NOT be used to select the shared secret used to authenticate the request. |
| `5.4:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS SERVER. Ze is a RADIUS CLIENT (a NAS): it sends Access-Requests and reads the reply. No file in the tree implements a RADIUS authentication server, and the only RADIUS listener ze runs is the RFC 5176 CoA/Disconnect port (internal/component/l2tp/plugins/authradius/coa.go), whose obligations belong to RFC 5176. It names the input the server must use instead, and repeats Section 3's sentence at site 3:4. | The source IP address of the Access-Request packet MUST be used to select the shared secret. |
| `5.6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Service-Type section stating for one attribute what Section 1.1 states in general. RFC2865-1.1-2 names both sections and site 1.1:3 maps it. | A NAS is not required to implement all of these service types, and MUST treat unknown or unsupported Service-Types as though an Access-Reject had been received instead. |
| `5.18:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a NAS that DISPLAYS several Reply-Messages to a user, a role ze does not implement. Both RADIUS paths surface at most one: FindAttr returns the first (internal/component/radius/authenticator.go, internal/component/l2tp/plugins/authradius/handler.go), so no display order exists to get wrong. | Multiple Reply-Message's MAY be included and if any are displayed, they MUST be displayed in the same order as they appear in the packet. |
| `5.18:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Reply-Message repeating the Filter-Id sentence. RFC2865-5.11-1 names Section 5.18 and site 5.11:1 maps it. | It is intended to be human readable, and MUST NOT affect operation of the protocol. |
| `5.22:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Framed-Route repeating the Filter-Id sentence. RFC2865-5.11-1 names Section 5.22 and site 5.11:1 maps it. | It is intended to be human readable and MUST NOT affect operation of the protocol. |
| `5.24:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a NAS that SUPPORTS challenge/response, a role ze does not implement. Section 4.4 provides for a NAS without it and states what it owes instead, which is RFC2865-4.4-1: treat the Access-Challenge as an Access-Reject. There is therefore no 'new Access-Request reply to that challenge' for ze to carry State in. The producer that would act as it if ze did is ze's RADIUS code, which is a client only: `Exchange` and `SendToServers` (`internal/component/radius/client.go`) send a request and match the reply, and `Authenticate` (`internal/component/radius/authenticator.go`) consumes the response. Nothing serves, proxies or re-authenticates. | This Attribute is available to be sent by the server to the client in an Access-Challenge and MUST be sent unmodified from the client to the server in the new Access-Request reply to that challenge, if any. |
| `5.24:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a NAS that performs the Termination-Action by sending a new Access-Request, a role ze does not implement: neither RADIUS path reads Termination-Action (attribute 29) and neither re-authenticates on session termination. The producer that would act as it if ze did is ze's RADIUS code, which is a client only: `Exchange` and `SendToServers` (`internal/component/radius/client.go`) send a request and match the reply, and `Authenticate` (`internal/component/radius/authenticator.go`) consumes the response. Nothing serves, proxies or re-authenticates. | If the NAS performs the Termination-Action by sending a new Access-Request upon termination of the current session, it MUST include the State attribute unchanged in that Access-Request. |
| `5.24:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | State stating what Section 5.25 states for Class in the same words. RFC2865-5.25-1 names both sections and site 5.25:1 maps it. | In either usage, the client MUST NOT interpret the attribute locally. |
| `5.26:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Vendor-Specific repeating the Filter-Id sentence. RFC2865-5.11-1 names Section 5.26 and site 5.11:1 maps it. | It MUST not affect the operation of the RADIUS protocol. |
| `5.26:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS SERVER. Ze is a RADIUS CLIENT (a NAS): it sends Access-Requests and reads the reply. No file in the tree implements a RADIUS authentication server, and the only RADIUS listener ze runs is the RFC 5176 CoA/Disconnect port (internal/component/l2tp/plugins/authradius/coa.go), whose obligations belong to RFC 5176. The sentence names its speaker: 'Servers not equipped to interpret the vendor-specific information sent by a client'. | Servers not equipped to interpret the vendor-specific information sent by a client MUST ignore it (although it may be reported). |
| `5.32:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The NAS-Identifier section restating the Section 4.1 rule. RFC2865-4.1-2 carries it and site 4.1:3 maps it. | Either NAS-IP-Address or NAS-Identifier MUST be present in an Access-Request packet. |
| `5.32:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS SERVER. Ze is a RADIUS CLIENT (a NAS): it sends Access-Requests and reads the reply. No file in the tree implements a RADIUS authentication server, and the only RADIUS listener ze runs is the RFC 5176 CoA/Disconnect port (internal/component/l2tp/plugins/authradius/coa.go), whose obligations belong to RFC 5176. It forbids one input to the server's shared-secret choice, as site 5.4:2 does for NAS-IP-Address. | Note that NAS-Identifier MUST NOT be used to select the shared secret used to authenticate the request. |
| `5.32:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS SERVER. Ze is a RADIUS CLIENT (a NAS): it sends Access-Requests and reads the reply. No file in the tree implements a RADIUS authentication server, and the only RADIUS listener ze runs is the RFC 5176 CoA/Disconnect port (internal/component/l2tp/plugins/authradius/coa.go), whose obligations belong to RFC 5176. It names the input the server must use instead. | The source IP address of the Access-Request packet MUST be used to select the shared secret. |
| `5.33:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a PROXY SERVER adding Proxy-State and the server returning it. the FORWARDING SERVER of proxy RADIUS. Ze never forwards a RADIUS request: internal/component/radius/client.go sends its own Access-Requests and internal/component/l2tp/plugins/authradius does the same for subscribers. Ze adds no Proxy-State and reads none. | This Attribute is available to be sent by a proxy server to another server when forwarding an Access-Request and MUST be returned unmodified in the Access-Accept, Access-Reject or Access-Challenge. |
| `5.33:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a PROXY SERVER stripping its own Proxy-State from a response. the FORWARDING SERVER of proxy RADIUS. Ze never forwards a RADIUS request: internal/component/radius/client.go sends its own Access-Requests and internal/component/l2tp/plugins/authradius does the same for subscribers. Ze adds no Proxy-State and reads none. | When the proxy server receives the response to its request, it MUST remove its own Proxy-State (the last Proxy- State in the packet) before forwarding the response to the NAS. |
| `5.33:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds whichever speaker ADDS a Proxy-State when forwarding a packet. the FORWARDING SERVER of proxy RADIUS. Ze never forwards a RADIUS request: internal/component/radius/client.go sends its own Access-Requests and internal/component/l2tp/plugins/authradius does the same for subscribers. Ze adds no Proxy-State and reads none. | If a Proxy-State Attribute is added to a packet when forwarding the packet, the Proxy-State Attribute MUST be added after any existing Proxy-State attributes. |
| `5.33:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Proxy-State repeating the Filter-Id sentence for the Proxy-States a speaker did not add. RFC2865-5.11-1 names Section 5.33 and site 5.11:1 maps it. | The content of any Proxy-State other than the one added by the current server should be treated as opaque octets and MUST NOT affect operation of the protocol. |
| `5.44:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Note 1 of the Table of Attributes restating Section 4.1's credential rule. RFC2865-4.1-3 carries it and site 4.1:4 maps it. | [Note 1] An Access-Request MUST contain either a User-Password or a CHAP-Password or State. |
| `5.44:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second sentence of Note 1, restating Section 4.1's both-credentials prohibition. RFC2865-4.1-4 carries it and site 4.1:5 maps it. | An Access-Request MUST NOT contain both a User-Password and a CHAP-Password. |
| `5.44:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Note 2 of the Table of Attributes restating Section 4.1's NAS identification rule. RFC2865-4.1-2 carries it and site 4.1:3 maps it. | [Note 2] An Access-Request MUST contain either a NAS-IP-Address or a NAS-Identifier (or both). |
| `0:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The legend of the Section 5.44 Table of Attributes, which the section parser split into a section of its own because the line begins with '0'. It defines what a '0' cell means and binds no speaker: 'this attribute' names none. The obligations it expands to are the table's per-attribute rows, and both ze Access-Request builders send only attributes the Request column permits. | This attribute MUST NOT be present in packet. |
| `9:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A Change Log entry describing a change made between RFC 2138 and this document. It quotes the Section 5 sentence at site 5:1, whose speaker is a proxy, and binds nobody itself. | If multiple Attributes with the same Type are present, the order of Attributes with the same Type MUST be preserved by any proxies. |

## Superseded

No document obsoletes RFC 2865, so its obligations are stated where they were written.

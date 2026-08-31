// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS attribute dictionary
// RFC: rfc/short/rfc2865.md -- base packet codes and attribute types
// RFC: rfc/short/rfc2866.md -- accounting packet codes and attribute types
// RFC: rfc/short/rfc5176.md -- CoA/Disconnect packet codes and Error-Cause values
// Related: packet.go -- packet encode/decode consuming these codes
// Related: client.go -- client transport using packet codes
// Related: attr.go -- attribute encode/decode helpers

package radius

// RADIUS packet codes (RFC 2865 Section 3, RFC 2866 Section 3, RFC 5176 Section
// 2.3). RFC 5176 assigns codes 40 to 45 in its Code field description; its
// Section 3 is Attributes.
const (
	CodeAccessRequest   = 1
	CodeAccessAccept    = 2
	CodeAccessReject    = 3
	CodeAccountingReq   = 4
	CodeAccountingResp  = 5
	CodeAccessChallenge = 11

	// RFC 5176 Section 3: Dynamic Authorization Extensions (CoA/DM).
	CodeDisconnectRequest = 40
	CodeDisconnectACK     = 41
	CodeDisconnectNAK     = 42
	CodeCoARequest        = 43
	CodeCoAACK            = 44
	CodeCoANAK            = 45
)

// RADIUS attribute type codes (RFC 2865 Section 5).
const (
	AttrUserName             = 1
	AttrUserPassword         = 2
	AttrCHAPPassword         = 3
	AttrNASIPAddress         = 4
	AttrNASPort              = 5
	AttrServiceType          = 6
	AttrFramedProtocol       = 7
	AttrFramedIPAddress      = 8
	AttrFramedIPNetmask      = 9
	AttrFilterID             = 11
	AttrFramedRoute          = 22 // RFC 2865 Section 5.22
	AttrReplyMessage         = 18
	AttrState                = 24 // RFC 2865 Section 5.24
	AttrClass                = 25 // RFC 2865 Section 5.25
	AttrSessionTimeout       = 27
	AttrIdleTimeout          = 28
	AttrCalledStationID      = 30
	AttrCallingStationID     = 31
	AttrNASIdentifier        = 32
	AttrProxyState           = 33 // RFC 2865 Section 5.33
	AttrAcctStatusType       = 40
	AttrAcctDelayTime        = 41
	AttrAcctInputOctets      = 42
	AttrAcctOutputOctets     = 43
	AttrAcctSessionID        = 44
	AttrAcctSessionTime      = 46
	AttrAcctInputPackets     = 47
	AttrAcctOutputPackets    = 48
	AttrAcctTerminateCause   = 49 // RFC 2866 Section 5.10
	AttrAcctMultiSessionID   = 50 // RFC 2866 Section 5.11
	AttrAcctInputGigawords   = 52
	AttrAcctOutputGigawords  = 53
	AttrEventTimestamp       = 55
	AttrCHAPChallenge        = 60
	AttrNASPortType          = 61
	AttrMessageAuthenticator = 80
	AttrAcctInterimInterval  = 85
	AttrNASPortID            = 87 // RFC 2869 Section 5.17: UTF-8 text, Length >= 3
	AttrFramedPool           = 88
	AttrChargeableUserID     = 89  // RFC 4372 Section 2.1
	AttrNASIPv6Address       = 95  // RFC 3162 Section 2.1
	AttrFramedInterfaceID    = 96  // RFC 3162 Section 2.2
	AttrFramedIPv6Prefix     = 97  // RFC 3162 Section 2.3
	AttrFramedIPv6Route      = 99  // RFC 6911 Section 3.2
	AttrErrorCause           = 101 // RFC 5176 Section 3.5 defines it; 3.6 only tables it
	AttrVendorSpecific       = 26
)

// Vendor IDs for vendor-specific attributes (RFC 2865 Section 5.26).
const (
	VendorCisco     = 9
	VendorMicrosoft = 311
	VendorHuawei    = 2011
	VendorJuniper   = 4874
	VendorNokia     = 6527
	VendorMikrotik  = 14988
)

// Cisco vendor-specific attribute types.
const (
	CiscoAVPair = 1
)

// Juniper ERX vendor-specific attribute types.
const (
	ERXIngressPolicyName = 10
	ERXEgressPolicyName  = 11
)

// Huawei vendor-specific attribute types.
const (
	HWSubscriberQoSProfile = 17
)

// Nokia vendor-specific attribute types.
const (
	AlcSubscriberQoSOverride = 126
)

// MikroTik vendor-specific attribute types.
const (
	MikrotikRateLimit = 8
)

// Microsoft vendor-specific attribute types.
const (
	MSCHAPChallenge = 11 // MS-CHAP-Challenge
	MSCHAP2Response = 25 // MS-CHAP2-Response
	MSCHAP2Success  = 26 // MS-CHAP2-Success
)

// Accounting Status-Type values (RFC 2866 Section 5.1).
const (
	AcctStatusStart         = 1
	AcctStatusStop          = 2
	AcctStatusInterimUpdate = 3
)

// Service-Type values (RFC 2865 Section 5.6).
const (
	ServiceTypeFramed = 2
)

// Framed-Protocol values (RFC 2865 Section 5.7).
const (
	FramedProtocolPPP = 1
)

// NAS-Port-Type values (RFC 2865 Section 5.41).
const (
	NASPortTypeVirtual = 5
)

// Error-Cause values (RFC 5176 Section 3.5). Section 3.6 is the table of
// attributes; Section 3.5 defines the Error-Cause attribute and its values.
const (
	ErrorCauseResidualSession      = 201
	ErrorCauseInvalidEAPPacket     = 202
	ErrorCauseUnsupportedAttribute = 401
	ErrorCauseMissingAttribute     = 402
	ErrorCauseNASIdentification    = 403
	ErrorCauseInvalidRequest       = 404
	ErrorCauseUnsupportedService   = 405
	ErrorCauseUnsupportedExtension = 406
	ErrorCauseSessionNotFound      = 503
	// ErrorCauseResourcesUnavailable (506) and
	// ErrorCauseMultiSessionUnsupported (508) are the two 500-series values ze
	// emits: the NAS could not carry out the authorization change, and the
	// identification attributes selected more than one session.
	ErrorCauseResourcesUnavailable    = 506
	ErrorCauseMultiSessionUnsupported = 508
)

// Wire constants.
const (
	HeaderLen        = 20 // Code(1) + ID(1) + Length(2) + Authenticator(16)
	AuthenticatorLen = 16
	MaxPacketLen     = 4096
	MinPacketLen     = HeaderLen
	MaxAttrLen       = 255
	MinAttrLen       = 3 // Type(1) + Length(1) + Value(1+)
)

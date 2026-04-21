// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS attribute dictionary
// Related: packet.go -- packet encode/decode consuming these codes
// Related: client.go -- client transport using packet codes
// Related: attr.go -- attribute encode/decode helpers

package radius

// RADIUS packet codes (RFC 2865 Section 3, RFC 2866 Section 3, RFC 5176 Section 3).
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
	AttrUserName          = 1
	AttrUserPassword      = 2
	AttrCHAPPassword      = 3
	AttrNASIPAddress      = 4
	AttrNASPort           = 5
	AttrServiceType       = 6
	AttrFramedProtocol    = 7
	AttrFramedIPAddress   = 8
	AttrFramedIPNetmask   = 9
	AttrFilterID          = 11
	AttrReplyMessage      = 18
	AttrSessionTimeout    = 27
	AttrCalledStationID   = 30
	AttrCallingStationID  = 31
	AttrNASIdentifier     = 32
	AttrAcctStatusType    = 40
	AttrAcctDelayTime     = 41
	AttrAcctInputOctets   = 42
	AttrAcctOutputOctets  = 43
	AttrAcctSessionID     = 44
	AttrAcctSessionTime   = 46
	AttrAcctInputPackets  = 47
	AttrAcctOutputPackets = 48
	AttrCHAPChallenge     = 60
	AttrNASPortType       = 61
	AttrFramedPool        = 88
	AttrErrorCause        = 101 // RFC 5176 Section 3.6
	AttrVendorSpecific    = 26
)

// Vendor IDs for vendor-specific attributes (RFC 2865 Section 5.26).
const (
	VendorMicrosoft = 311
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

// Error-Cause values (RFC 5176 Section 3.6).
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

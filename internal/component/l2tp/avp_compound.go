// Design: docs/architecture/wire/l2tp.md — compound AVP value shapes
// RFC: rfc/short/rfc2661.md — RFC 2661 Sections 4.4.2, 4.4.4, 5.7, 5.8
// Related: avp.go — generic AVP header/value helpers used here

package l2tp

import "encoding/binary"

// ProtocolVersionValue is the wire encoding for AVPProtocolVersion.
// RFC 2661 Section 4.4.2: "the wire bytes are 0x01 0x00" (version 1, rev 0).
var ProtocolVersionValue = [2]byte{0x01, 0x00}

// ResultCodeValue is the decoded Result Code AVP body.
//
// RFC 2661 Sections 4.4.2 (StopCCN) and 5.7 (CDN).
//
//	+---------+--------+----------+----------+
//	| Result  | Error  | Message  |          |
//	| (u16)   | (u16)* | string*  |          |
//	+---------+--------+----------+----------+
//
// Fields marked * are optional. ErrorPresent and MessagePresent indicate
// whether those optional fields were present in the parsed value.
type ResultCodeValue struct {
	Result         uint16
	Error          uint16
	ErrorPresent   bool
	Message        string
	MessagePresent bool
}

// ReadResultCode parses the Result Code AVP body.
func ReadResultCode(value []byte) (ResultCodeValue, error) {
	if len(value) < 2 {
		return ResultCodeValue{}, ErrInvalidAVPLen
	}
	rc := ResultCodeValue{Result: binary.BigEndian.Uint16(value[:2])}
	if len(value) == 2 {
		return rc, nil
	}
	if len(value) < 4 {
		return ResultCodeValue{}, ErrInvalidAVPLen
	}
	rc.Error = binary.BigEndian.Uint16(value[2:4])
	rc.ErrorPresent = true
	if len(value) > 4 {
		rc.Message = string(value[4:])
		rc.MessagePresent = true
	}
	return rc, nil
}

// WriteAVPResultCode writes a Result Code AVP into buf at off. If rc.ErrorPresent
// is false, only the 2-byte Result field is written. If rc.MessagePresent is true,
// the advisory message is appended after Error. Returns bytes written.
func WriteAVPResultCode(buf []byte, off int, mandatory bool, rc ResultCodeValue) int {
	valueOff := off + AVPHeaderLen
	binary.BigEndian.PutUint16(buf[valueOff:], rc.Result)
	valueLen := 2
	if rc.ErrorPresent {
		binary.BigEndian.PutUint16(buf[valueOff+2:], rc.Error)
		valueLen = 4
		if rc.MessagePresent {
			n := copy(buf[valueOff+4:], rc.Message)
			valueLen = 4 + n
		}
	}
	flags := AVPFlags(0)
	if mandatory {
		flags |= FlagMandatory
	}
	total := AVPHeaderLen + valueLen
	WriteAVPHeader(buf, off, flags, 0, AVPResultCode, total)
	return total
}

// Q931CauseValue is the decoded Q.931 Cause Code AVP body.
// RFC 2661 Section 4.4.4:
//
//	+--------+------+----------+
//	| Cause  | Msg  | Advisory |
//	| (u16)  | (u8) | string*  |
//	+--------+------+----------+
type Q931CauseValue struct {
	Cause           uint16
	Msg             uint8
	Advisory        string
	AdvisoryPresent bool
}

// ReadQ931Cause parses the Q.931 Cause Code AVP body.
func ReadQ931Cause(value []byte) (Q931CauseValue, error) {
	if len(value) < 3 {
		return Q931CauseValue{}, ErrInvalidAVPLen
	}
	v := Q931CauseValue{
		Cause: binary.BigEndian.Uint16(value[:2]),
		Msg:   value[2],
	}
	if len(value) > 3 {
		v.Advisory = string(value[3:])
		v.AdvisoryPresent = true
	}
	return v, nil
}

// WriteAVPQ931Cause writes a Q.931 Cause Code AVP. Returns bytes written.
func WriteAVPQ931Cause(buf []byte, off int, mandatory bool, v Q931CauseValue) int {
	valueOff := off + AVPHeaderLen
	binary.BigEndian.PutUint16(buf[valueOff:], v.Cause)
	buf[valueOff+2] = v.Msg
	valueLen := 3
	if v.AdvisoryPresent {
		n := copy(buf[valueOff+3:], v.Advisory)
		valueLen = 3 + n
	}
	flags := AVPFlags(0)
	if mandatory {
		flags |= FlagMandatory
	}
	total := AVPHeaderLen + valueLen
	WriteAVPHeader(buf, off, flags, 0, AVPQ931CauseCode, total)
	return total
}

// CallErrorsValue is the decoded Call Errors AVP body.
// RFC 2661 Section 5.8: fixed 26-byte layout.
type CallErrorsValue struct {
	CRCErrors        uint32
	FramingErrors    uint32
	HardwareOverruns uint32
	BufferOverruns   uint32
	TimeoutErrors    uint32
	AlignmentErrors  uint32
}

// ReadCallErrors parses the Call Errors AVP body. RFC 2661 Section 5.8.1.
func ReadCallErrors(value []byte) (CallErrorsValue, error) {
	if len(value) != 26 {
		return CallErrorsValue{}, ErrInvalidAVPLen
	}
	return CallErrorsValue{
		// value[0:2] is reserved (always zero per RFC).
		CRCErrors:        binary.BigEndian.Uint32(value[2:]),
		FramingErrors:    binary.BigEndian.Uint32(value[6:]),
		HardwareOverruns: binary.BigEndian.Uint32(value[10:]),
		BufferOverruns:   binary.BigEndian.Uint32(value[14:]),
		TimeoutErrors:    binary.BigEndian.Uint32(value[18:]),
		AlignmentErrors:  binary.BigEndian.Uint32(value[22:]),
	}, nil
}

// WriteAVPCallErrors writes a Call Errors AVP (26-byte fixed layout).
func WriteAVPCallErrors(buf []byte, off int, mandatory bool, v CallErrorsValue) int {
	valueOff := off + AVPHeaderLen
	// Reserved uint16 at value[0:2] MUST be zero.
	binary.BigEndian.PutUint16(buf[valueOff:], 0)
	binary.BigEndian.PutUint32(buf[valueOff+2:], v.CRCErrors)
	binary.BigEndian.PutUint32(buf[valueOff+6:], v.FramingErrors)
	binary.BigEndian.PutUint32(buf[valueOff+10:], v.HardwareOverruns)
	binary.BigEndian.PutUint32(buf[valueOff+14:], v.BufferOverruns)
	binary.BigEndian.PutUint32(buf[valueOff+18:], v.TimeoutErrors)
	binary.BigEndian.PutUint32(buf[valueOff+22:], v.AlignmentErrors)
	flags := AVPFlags(0)
	if mandatory {
		flags |= FlagMandatory
	}
	total := AVPHeaderLen + 26
	WriteAVPHeader(buf, off, flags, 0, AVPCallErrors, total)
	return total
}

// ACCMValue is the decoded ACCM AVP body.
// RFC 2661 Section 5.8.2: fixed 10-byte layout (reserved u16 + sendACCM u32 + recvACCM u32).
type ACCMValue struct {
	SendACCM uint32
	RecvACCM uint32
}

// ReadACCM parses the ACCM AVP body.
func ReadACCM(value []byte) (ACCMValue, error) {
	if len(value) != 10 {
		return ACCMValue{}, ErrInvalidAVPLen
	}
	return ACCMValue{
		SendACCM: binary.BigEndian.Uint32(value[2:]),
		RecvACCM: binary.BigEndian.Uint32(value[6:]),
	}, nil
}

// WriteAVPACCM writes an ACCM AVP (10-byte fixed layout).
func WriteAVPACCM(buf []byte, off int, mandatory bool, v ACCMValue) int {
	valueOff := off + AVPHeaderLen
	binary.BigEndian.PutUint16(buf[valueOff:], 0)
	binary.BigEndian.PutUint32(buf[valueOff+2:], v.SendACCM)
	binary.BigEndian.PutUint32(buf[valueOff+6:], v.RecvACCM)
	flags := AVPFlags(0)
	if mandatory {
		flags |= FlagMandatory
	}
	total := AVPHeaderLen + 10
	WriteAVPHeader(buf, off, flags, 0, AVPACCM, total)
	return total
}

// ProxyAuthenIDValue is the two-byte Proxy Authen ID AVP value
// (reserved u8 + CHAP ID u8). RFC 2661 Section 4.4.6.
type ProxyAuthenIDValue struct {
	ChapID uint8
}

// ReadProxyAuthenID parses the Proxy Authen ID AVP body.
func ReadProxyAuthenID(value []byte) (ProxyAuthenIDValue, error) {
	if len(value) != 2 {
		return ProxyAuthenIDValue{}, ErrInvalidAVPLen
	}
	return ProxyAuthenIDValue{ChapID: value[1]}, nil
}

// WriteAVPProxyAuthenID writes the Proxy Authen ID AVP.
func WriteAVPProxyAuthenID(buf []byte, off int, mandatory bool, v ProxyAuthenIDValue) int {
	valueOff := off + AVPHeaderLen
	buf[valueOff] = 0 // reserved
	buf[valueOff+1] = v.ChapID
	flags := AVPFlags(0)
	if mandatory {
		flags |= FlagMandatory
	}
	total := AVPHeaderLen + 2
	WriteAVPHeader(buf, off, flags, 0, AVPProxyAuthenID, total)
	return total
}

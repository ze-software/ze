// Design: docs/architecture/provisioning/dhcp-server.md -- option overload on receive (RFC 2131 Section 4.1)
// Related: handler.go -- the option readers built on findOption
// RFC: rfc/short/rfc2131.md

package dhcpserver

// BOOTP fixed-header field offsets (RFC 2131 Section 2, Figure 1).
const (
	snameStart   = 44  // sname: 64 octets, optional server host name
	fileStart    = 108 // file: 128 octets, boot file name
	fileEnd      = 236 // the magic cookie follows the file field
	optionsStart = 240 // the options field follows the magic cookie
)

// optOverload is the option overload option (RFC 2132 Section 9.3): its single
// octet says which of the file (1), sname (2) or both (3) fields carry options.
const optOverload = 52

// Option overload values (RFC 2132 Section 9.3).
const (
	overloadFile  = 1
	overloadSname = 2
	overloadBoth  = 3
)

// findOption returns the data of the first option with the given code in the
// order RFC 2131 Section 4.1 interprets the fields: options, then file, then
// sname, the last two only when the options field's option overload says so.
// ok is false when no field in use holds the code, including for a packet too
// short to reach the options field.
//
// The returned slice aliases pkt. Every field is walked within its own bounds,
// so a hostile length octet ends the walk of that field and never reads past it.
// Safe for concurrent use: it only reads pkt.
func findOption(pkt []byte, code byte) ([]byte, bool) {
	if len(pkt) < optionsStart {
		return nil, false
	}
	// RFC 2131 Section 4.1: "The options in the 'options' field MUST be
	// interpreted first, so that any 'option overload' options may be
	// interpreted."
	if data, ok := findFieldOption(pkt[optionsStart:], code); ok {
		return data, true
	}
	overload := overloadValue(pkt[optionsStart:])
	// RFC 2131 Section 4.1: "The 'file' field MUST be interpreted next (if the
	// 'option overload' option indicates that the 'file' field contains DHCP
	// options), followed by the 'sname' field."
	if overload == overloadFile || overload == overloadBoth {
		if data, ok := findFieldOption(pkt[fileStart:fileEnd], code); ok {
			return data, true
		}
	}
	if overload == overloadSname {
		return findFieldOption(pkt[snameStart:fileStart], code)
	}
	if overload == overloadBoth {
		return findFieldOption(pkt[snameStart:fileStart], code)
	}
	return nil, false
}

// overloadValue reads the option overload value from the options field. It
// returns 0, which opens no field, when the option is absent or is not exactly
// one octet long. An option 52 inside file or sname is never consulted.
func overloadValue(options []byte) byte {
	data, ok := findFieldOption(options, optOverload)
	if !ok {
		return 0
	}
	if len(data) != 1 {
		return 0
	}
	return data[0]
}

// findFieldOption walks one option field and returns the data of the first
// option with the given code. The loop is bounded by len(field), and an option
// whose declared length runs past the end of the field is not read: RFC 2131
// Section 4.1 requires every option to be entirely contained in its field, so
// the walk stops there rather than taking bytes from the next field.
func findFieldOption(field []byte, code byte) ([]byte, bool) {
	for i := 0; i < len(field); {
		current := field[i]
		if current == optEnd {
			return nil, false
		}
		if current == optPad {
			i++
			continue
		}
		if i+2 > len(field) {
			return nil, false
		}
		end := i + 2 + int(field[i+1])
		if end > len(field) {
			return nil, false
		}
		if current == code {
			return field[i+2 : end], true
		}
		i = end
	}
	return nil, false
}

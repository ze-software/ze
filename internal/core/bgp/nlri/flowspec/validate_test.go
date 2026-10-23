package flowspec

import (
	"encoding/hex"
	"testing"
)

// validateCorpus holds FlowSpec NLRI values, length field included, that the
// validator and the parser must judge alike: well-formed ones and one of each
// refusal the shared checks make.
var validateCorpus = []struct {
	name string
	fam  Family
	hex  string
	good bool
}{
	{"ipv4 dest prefix", Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}, "0501180A0000", true},
	{"ipv4 prefix protocol port", Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}, "0C01180A0000038106059100" + "50", true},
	{"ipv6 corpus 4.2 well formed", Family{AFI: AFIIPv6, SAFI: SAFIFlowSpec},
		"210180002A020B80001500007ACA39FFFEAEA87A" + "02807801" + "0381110B81650D9107DD", true},
	{"ipv6 corpus 4.2 sixteen pattern octets", Family{AFI: AFIIPv6, SAFI: SAFIFlowSpec},
		"300180002A020B80001500007ACA39FFFEAEA87A" + "028078000000000000000000000000000000" + "01" + "0381110B81650D9107DD", false},
	{"unknown component type", Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}, "0400200A0A", false},
	{"length overruns", Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}, "060220CAFF", false},
	{"empty", Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}, "", false},
	{"duplicate type", Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}, "0603810603810" + "6", false},
	{"types out of order", Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}, "0705910050038106", false},
	{"numeric list without end bit", Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}, "03030106", false},
	{"dscp two octet value", Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}, "040B910001", false},
	{"flow label on ipv4", Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}, "030D8101", false},
	{"ipv4 prefix longer than 32", Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}, "0601210A000000", false},
	{"ipv6 offset at prefix length", Family{AFI: AFIIPv6, SAFI: SAFIFlowSpec}, "0401080800", false},
	{"vpn well formed", Family{AFI: AFIIPv4, SAFI: SAFIFlowSpecVPN}, "0D" + "0000FDE800000001" + "01180A0000", true},
	{"vpn shorter than its rd", Family{AFI: AFIIPv4, SAFI: SAFIFlowSpecVPN}, "04" + "0000FDE8", false},
	{"vpn bad component", Family{AFI: AFIIPv4, SAFI: SAFIFlowSpecVPN}, "0A" + "0000FDE800000001" + "0006", false},
}

// judgeLikeParser runs the validator and the parser of fam over data.
func judgeLikeParser(fam Family, data []byte) (validateErr, parseErr error) {
	if fam.SAFI == SAFIFlowSpecVPN {
		_, parseErr = ParseFlowSpecVPN(fam, data)
		return ValidateFlowSpecVPN(fam, data), parseErr
	}
	_, parseErr = ParseFlowSpec(fam, data)
	return ValidateFlowSpec(fam, data), parseErr
}

// TestValidateFlowSpecAgreesWithParse judges each corpus value with the
// validator and with the parser.
//
// VALIDATES: ValidateFlowSpec and ValidateFlowSpecVPN refuse exactly what
// ParseFlowSpec and ParseFlowSpecVPN refuse, with the same error, on a
// well-formed value and on one value per refusal the shared checks make.
// PREVENTS: the receive path admitting an NLRI the plugin then cannot decode
// (the `wire[ipv6/flow](49 bytes)` announce), or resetting a session over one
// it could.
func TestValidateFlowSpecAgreesWithParse(t *testing.T) {
	for _, tt := range validateCorpus {
		t.Run(tt.name, func(t *testing.T) {
			data, err := hex.DecodeString(tt.hex)
			if err != nil {
				t.Fatalf("bad hex: %v", err)
			}
			validateErr, parseErr := judgeLikeParser(tt.fam, data)
			if (validateErr == nil) != tt.good {
				t.Fatalf("validate: %v, want well-formed=%v", validateErr, tt.good)
			}
			if (parseErr == nil) != tt.good {
				t.Fatalf("parse: %v, want well-formed=%v", parseErr, tt.good)
			}
			if !tt.good && validateErr.Error() != parseErr.Error() {
				t.Fatalf("validate refused with %q, parse with %q", validateErr, parseErr)
			}
		})
	}
}

// TestValidateFlowSpecAllocatesNothing measures the validator over each
// well-formed corpus value.
//
// VALIDATES: a well-formed FlowSpec NLRI is judged with zero allocations, so
// the receive-path syntax rule costs no garbage per UPDATE.
// PREVENTS: a decode (slices, component structs) creeping back into the walk.
func TestValidateFlowSpecAllocatesNothing(t *testing.T) {
	for _, tt := range validateCorpus {
		if !tt.good {
			continue
		}
		data, err := hex.DecodeString(tt.hex)
		if err != nil {
			t.Fatalf("bad hex: %v", err)
		}
		allocs := testing.AllocsPerRun(100, func() {
			if tt.fam.SAFI == SAFIFlowSpecVPN {
				_ = ValidateFlowSpecVPN(tt.fam, data) //nolint:errcheck // the agreement test judges the answer
				return
			}
			_ = ValidateFlowSpec(tt.fam, data) //nolint:errcheck // the agreement test judges the answer
		})
		if allocs != 0 {
			t.Fatalf("%s: %v allocations per validation, want 0", tt.name, allocs)
		}
	}
}

// FuzzValidateFlowSpecAgreesWithParse feeds arbitrary bytes to the validator
// and the parser of each FlowSpec family.
//
// VALIDATES: the two refuse the same inputs, with the same error.
// PREVENTS: the validator and the parser drifting apart on a check one of them
// gains.
// SECURITY: FlowSpec bytes come from untrusted BGP UPDATE NLRI.
func FuzzValidateFlowSpecAgreesWithParse(f *testing.F) {
	for _, tt := range validateCorpus {
		data, err := hex.DecodeString(tt.hex)
		if err != nil {
			f.Fatalf("bad hex: %v", err)
		}
		f.Add(data, uint8(tt.fam.AFI), tt.fam.SAFI == SAFIFlowSpecVPN)
	}
	f.Fuzz(func(t *testing.T, data []byte, afi uint8, vpn bool) {
		fam := Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}
		if afi%2 == 0 {
			fam.AFI = AFIIPv6
		}
		if vpn {
			fam.SAFI = SAFIFlowSpecVPN
		}
		validateErr, parseErr := judgeLikeParser(fam, data)
		if (validateErr == nil) != (parseErr == nil) {
			t.Fatalf("%x: validate %v, parse %v", data, validateErr, parseErr)
		}
		if validateErr != nil && validateErr.Error() != parseErr.Error() {
			t.Fatalf("%x: validate refused with %q, parse with %q", data, validateErr, parseErr)
		}
	})
}

// Design: docs/architecture/doctor-and-health-checks.md -- MUP segment discovery diagnostic codes
// Related: rfc7606.go -- MUP UpdateCheck raising these codes on unresolved or mismatched locators
// Related: register.go -- registers code metadata and event codes at init

package mup

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	// reportSourceMUP identifies the MUP NLRI plugin on the daemon report bus.
	reportSourceMUP = "bgp-nlri-mup"

	// codeDoctorBGPMUPLocatorUnresolved is raised on the report bus and logged
	// whenever a received MUP DSD or ISD route is kept unverified because the
	// originator of its Prefix-SID locator cannot be resolved from the IGP.
	codeDoctorBGPMUPLocatorUnresolved = "doctor-bgp-mup-locator-unresolved"

	// codeDoctorBGPMUPLocatorMismatch is raised on the report bus and logged
	// whenever a received MUP DSD or ISD route is treated as withdrawn because
	// its MP_REACH next hop or ISD Address does not belong to the proven IGP
	// originator of its Prefix-SID locator.
	codeDoctorBGPMUPLocatorMismatch = "doctor-bgp-mup-locator-mismatch"

	fixBGPMUPLocatorUnresolved = "ze show isis database"
	fixBGPMUPLocatorMismatch   = "ze show bgp peer list"
)

var mupDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeDoctorBGPMUPLocatorUnresolved,
		Title: "BGP MUP segment discovery locator originator could not be resolved",
		Description: "A received BGP-MUP Direct Segment Discovery (DSD) or Interwork Segment Discovery (ISD) route " +
			"was kept unverified because the originator of its Prefix-SID SRv6 locator could not be resolved from " +
			"the IGP locator-originator index (for example: IS-IS is not configured or stopped, no advertised " +
			"TLV 27 locator covers the SID, the locator is anycast or re-advertised without a matching IPv6 Source " +
			"Router ID, Prefix Attribute Flags are absent, the MP_REACH next-hop length is not comparable, or the " +
			"originator LSP carries undecoded TLV 237). Check the IS-IS LSDB with ze show isis database and inspect " +
			"active warnings with ze show warnings.",
		Examples: []string{
			fixBGPMUPLocatorUnresolved,
			"ze show warnings",
			"ze show diagnostic " + codeDoctorBGPMUPLocatorUnresolved,
		},
	},
	{
		Code:  codeDoctorBGPMUPLocatorMismatch,
		Title: "BGP MUP segment discovery route withdrawn on locator originator mismatch",
		Description: "A received BGP-MUP Direct Segment Discovery (DSD) or Interwork Segment Discovery (ISD) route " +
			"was treated as withdrawn (RFC 7606) because its MP_REACH_NLRI next hop or ISD Address field did not " +
			"match any IPv6 or IPv4 address or prefix held by the proven IS-IS originator of its Prefix-SID SRv6 locator " +
			"(draft-ietf-bess-mup-safi Sections 3.3.3 and 3.3.6). Inspect the peer's BGP announcements and IS-IS " +
			"locator advertisements with ze show bgp peer list and ze show isis database.",
		Examples: []string{
			fixBGPMUPLocatorMismatch,
			fixBGPMUPLocatorUnresolved,
			"ze show errors",
			"ze show diagnostic " + codeDoctorBGPMUPLocatorMismatch,
		},
	},
}

func registerMUPDiagnosticCodes() {
	for _, meta := range mupDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: mup: diagnostic code registration failed: " + err.Error())
		}
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorBGPMUPLocatorUnresolved}); err != nil {
		panic("BUG: mup: unresolved locator event code registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorBGPMUPLocatorMismatch}); err != nil {
		panic("BUG: mup: locator mismatch event code registration failed: " + err.Error())
	}
}

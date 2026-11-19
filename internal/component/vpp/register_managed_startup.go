// Design: docs/guide/vpp.md -- managed startup file diagnostic ownership
package vpp

import "github.com/ze-software/ze/internal/core/diagnostic"

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{Code: doctorVPPStartupCLIFileCode, Title: "VPP startup CLI file is unavailable",
		Description: "Managed startup checks an explicitly configured startup-cli-file before starting VPP. The path must be an absolute native token naming a readable regular file. A missing, unreadable or nonregular original is refused before child startup. Supply that file in the VPP process filesystem or delete vpp startup-cli-file; external supervision does not inspect this file. Native VPP owns execution of the command program.",
		Examples:    []string{"ze doctor", "ze show diagnostic " + doctorVPPStartupCLIFileCode}, HostVerdict: true}); err != nil {
		panic("BUG: cannot register VPP startup CLI diagnostic: " + err.Error())
	}
	if err := diagnostic.RegisterDoctorCheck(managedStartupCLIDoctorCheck()); err != nil {
		panic("BUG: cannot register VPP startup CLI doctor: " + err.Error())
	}
}

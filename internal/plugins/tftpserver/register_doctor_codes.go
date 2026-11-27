// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/tftpserver.
// Related: config_tx.go -- listenInterface resolve-fallback warning site
// Related: handler.go -- runTransfer dial-failure warning site
// Related: window_send.go -- sendWindow file-read-failure warning site

package tftpserver

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorTFTPInterfaceResolveFallback = "doctor-tftp-interface-resolve-fallback"
	codeDoctorTFTPTransferDialFailed       = "doctor-tftp-transfer-dial-failed"
	codeDoctorTFTPFileReadFailed           = "doctor-tftp-file-read-failed"

	exampleDoctorJSON     = "ze doctor --json"
	exampleShowWarnings   = "ze show warnings"
	exampleShowErrors     = "ze show errors"
	exampleShowDiagPrefix = "ze show diagnostic "
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeDoctorTFTPInterfaceResolveFallback,
		Title: "TFTP server could not resolve a logical listen interface",
		Description: "The TFTP server could not resolve a configured listen-interface through the interface resolver " +
			"and fell back to binding the literal interface name. If an os-name or mac-match selector maps the logical " +
			"interface to a different kernel device, the listener may bind the wrong device or fail. Check interface " +
			"selectors with `ze show interface` and `ze show doctor`.",
		Examples: []string{exampleDoctorJSON, exampleShowWarnings, exampleShowDiagPrefix + codeDoctorTFTPInterfaceResolveFallback},
	},
	{
		Code:  codeDoctorTFTPTransferDialFailed,
		Title: "TFTP server could not open a transfer UDP socket",
		Description: "The TFTP server accepted a read request but could not dial a UDP transfer socket back to the " +
			"client address, so the client received no data blocks and will time out. Check routing, firewall rules, " +
			"and socket limits with `ze show interface` and `ze show doctor`.",
		Examples: []string{exampleDoctorJSON, exampleShowErrors, exampleShowDiagPrefix + codeDoctorTFTPTransferDialFailed},
	},
	{
		Code:  codeDoctorTFTPFileReadFailed,
		Title: "TFTP server failed reading a file block during transfer",
		Description: "The TFTP server encountered a local file I/O error while reading a DATA block for an active " +
			"client transfer and aborted the transfer. Check storage health and file permissions under the configured " +
			"TFTP root-directory with `ze show doctor`.",
		Examples: []string{exampleDoctorJSON, exampleShowErrors, exampleShowDiagPrefix + codeDoctorTFTPFileReadFailed},
	},
}

// doctorEventCodes declares the transient report-bus event codes this package raises.
var doctorEventCodes = []diagnostic.EventCode{
	{Code: codeDoctorTFTPInterfaceResolveFallback},
	{Code: codeDoctorTFTPTransferDialFailed},
	{Code: codeDoctorTFTPFileReadFailed},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: tftpserver: doctor code registration refused: " + err.Error())
		}
	}
	for _, ev := range doctorEventCodes {
		if err := diagnostic.RegisterEventCode(ev); err != nil {
			panic("BUG: tftpserver: doctor event code registration refused: " + err.Error())
		}
	}
}

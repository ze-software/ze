// Design: docs/architecture/pki/tls-listeners.md -- each looking-glass entry chooses HTTPS or HTTP, one rule for start, reload and doctor
//
// Build-tag-gated registration of the doctor check that reports what the
// looking-glass listeners serve. Compiled only under //go:build ze_lg, like the
// service factory (register_lg.go): a binary without the looking glass has no
// listener to report on.
//
// Related: lg_doctor.go -- lgListenerDiagnostics, the report, and its codes

//go:build ze_lg

package hub

import (
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// lgDoctorExample is the command every looking-glass listener code names first.
const lgDoctorExample = "ze doctor"

// lgDoctorCodes is the explanation metadata of the looking-glass listener codes.
var lgDoctorCodes = []diagnostic.CodeMeta{
	{
		Code:  codeDoctorLGListenUnreadable,
		Title: "ze.looking-glass.listen does not parse",
		Description: "The environment variable ze.looking-glass.listen names the addresses the looking glass binds, and its value is not a list of listen endpoints. " +
			"ze refuses to start, and a reload is refused. Set it to endpoints such as 0.0.0.0:8443, or unset it and name the addresses in `environment looking-glass server`.",
		Examples: []string{lgDoctorExample, "ze show diagnostic doctor-lg-listen-unreadable"},
	},
	{
		Code:  codeDoctorLGTLSUnservable,
		Title: "Looking-glass server entry says tls true with no certificate",
		Description: "A looking-glass server entry says `tls true`, no certificate is named, and the self-signed certificate is kept in the database store, which the daemon runs without. " +
			"ze refuses to start, and a reload is refused. Name a pki certificate in `environment looking-glass certificate`, or run `ze init` and restart ze, or write `tls false` in that server entry to serve HTTP.",
		Examples:     []string{lgDoctorExample, "ze init", "ze show diagnostic doctor-lg-tls-unservable"},
		RelatedCodes: []string{codeDoctorLGPlainHTTP},
	},
	{
		Code:  codeDoctorLGPlainHTTP,
		Title: "Looking-glass server serves plain HTTP for want of a certificate",
		Description: "A looking-glass server entry left `tls` unset, which asks for HTTPS, but no certificate is named and the daemon runs without the database store that keeps the self-signed certificate, so that listener serves plain HTTP. " +
			"Run `ze init` and restart ze to serve HTTPS, or write `tls false` in that server entry to serve HTTP on purpose.",
		Examples:     []string{lgDoctorExample, "ze init", "ze show diagnostic doctor-lg-plain-http"},
		RelatedCodes: []string{codeDoctorLGTLSUnservable},
	},
}

func init() {
	for _, meta := range lgDoctorCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: hub: looking-glass doctor code registration refused: " + err.Error())
		}
	}
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name:         "lg-listener-protocol",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        728,
		Component:    "lg",
		Dependencies: []string{"storage"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeDoctorLGListenUnreadable, codeDoctorLGTLSUnservable, codeDoctorLGPlainHTTP},
		Check:        checkLGListeners,
	}); err != nil {
		panic("BUG: hub: looking-glass listener doctor registration refused: " + err.Error())
	}
}

// checkLGListeners is the doctor check: the post-config phase hands it the
// parsed tree, and the store doctor opened, which is absent exactly when the
// daemon would run without one.
func checkLGListeners(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, isTree := ctx.Tree.(*zeconfig.Tree)
	if !isTree {
		panic("BUG: hub: the looking-glass listener doctor check ran without a parsed config tree")
	}
	return lgListenerDiagnostics(tree, ctx.Store != nil)
}

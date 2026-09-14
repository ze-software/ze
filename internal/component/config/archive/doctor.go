// Design: docs/guide/config-archive.md -- the readiness check this component owns
// Overview: register.go -- the init() that installs the registration below
// Related: archive.go -- ExtractConfigs and ToHTTP, the parse and the upload this check stands in front of
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. A check that probes archive destinations belongs to
// the component that uploads to them (ai/patterns/registration.md, "Doctor
// Check Registry"), so it is owned here now and dropping this component drops
// its check with it.

package archive

import (
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeArchiveUnreachable names an HTTP archive destination that answers no
// HEAD. internal/core/diagnostic/codes.go declares it, so
// `ze explain doctor-archive-unreachable` answers.
const codeArchiveUnreachable = "doctor-archive-unreachable"

// archiveProbeTimeout bounds one destination probe. DoctorProbeTimeout can
// only shorten it.
const archiveProbeTimeout = 5 * time.Second

// archiveHTTPHead is the probe checkArchiveDestinations runs. It is a variable
// so a test can stand in an unreachable destination; nothing else assigns it.
var archiveHTTPHead = diagnostic.DoctorHTTPReachable

// archiveDoctorCheck describes the check. register.go registers it.
//
// Order 2250 reproduces the sequence the doctor runner printed before the
// check moved onto the registry: it ran right after the update backend check
// at 2240 (internal/component/config/system/doctor.go) and before the
// writable-destinations check at 2270
// (internal/component/doctor/doctor_checks.go).
var archiveDoctorCheck = diagnostic.DoctorCheck{
	Name:         "archive-destinations",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2250,
	Component:    "archive",
	Dependencies: []string{"archive-server"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeArchiveUnreachable},
	Check:        checkArchiveDestinations,
}

// checkArchiveDestinations warns for every http:// or https:// archive
// destination that answers no HEAD. A file:// destination is not probed here;
// its directory is the writable-destinations check's business.
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkArchiveDestinations(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}

	var diags []diagnostic.Diagnostic
	for _, ac := range ExtractConfigs(tree) {
		if ac.Location == "" {
			continue
		}
		if !strings.HasPrefix(ac.Location, schemeHTTP+"://") && !strings.HasPrefix(ac.Location, schemeHTTPS+"://") {
			continue
		}
		if err := archiveHTTPHead(ac.Location, diagnostic.DoctorProbeTimeout(archiveProbeTimeout)); err != nil {
			var tb textbuf.Buffer
			diags = append(diags, diagnostic.Diagnostic{
				Code:     codeArchiveUnreachable,
				Severity: diagnostic.SeverityWarning,
				Message:  tb.Str("archive ").Str(ac.Name).Str(": location unreachable: ").Err(err).String(),
				Path:     ac.Location,
			})
		}
	}
	return diags
}

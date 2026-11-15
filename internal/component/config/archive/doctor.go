// Design: docs/guide/config-archive.md -- the readiness check this component owns
// Overview: register.go -- the init() that installs the registration below
// Related: archive.go -- ExtractConfigs, ToHTTP and archiveClient, the parse, the upload and the redirect policy this check stands in front of
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. A check that probes archive destinations belongs to
// the component that uploads to them (ai/patterns/registration.md, "Doctor
// Check Registry"), so it is owned here now and dropping this component drops
// its check with it.

package archive

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeArchiveUnreachable names an HTTP archive destination that answers no
// HEAD. internal/core/diagnostic/codes.go declares it, so
// `ze show diagnostic doctor-archive-unreachable` answers.
const codeArchiveUnreachable = "doctor-archive-unreachable"

// codeArchiveRedirected names an HTTP archive destination that answers the
// probe with a redirect. ToHTTP follows none, so every upload to it fails.
const codeArchiveRedirected = "doctor-archive-redirected"

// codeArchiveInvalid names an archive entry ExtractConfigs refuses, which a
// validated config cannot hold. The check reports it rather than probing
// nothing in silence.
const codeArchiveInvalid = "doctor-archive-invalid"

// archiveProbeTimeout bounds one destination probe. DoctorProbeTimeout can
// only shorten it.
const archiveProbeTimeout = 5 * time.Second

// archiveHTTPHead is the probe checkArchiveDestinations runs. It is a variable
// so a test can stand in an unreachable destination; nothing else assigns it.
var archiveHTTPHead = probeArchiveDestination

// probeArchiveDestination sends an HTTP HEAD to location inside timeout,
// through archiveClient, which follows no redirect, the same as the upload.
// Any answer counts as reachable, whatever its status, except a redirect: the
// upload would refuse it, so the probe answers it as a *redirectedDestination.
// A 404 from a live server is a fact about the path rather than about
// reachability (diagnostic.DoctorHTTPReachable, which follows redirects, makes
// the same call for every other destination).
func probeArchiveDestination(location string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, location, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := archiveClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close() //nolint:errcheck // a HEAD answer has no body to lose
	if redirect, ok := redirectOf(resp); ok {
		return &redirectedDestination{redirect: redirect}
	}
	return nil
}

// redirectedDestination is the probe's answer for a destination that answered
// with a redirect.
type redirectedDestination struct {
	redirect uploadRedirect
}

func (d *redirectedDestination) Error() string {
	return "answered HTTP " + strconv.Itoa(d.redirect.status) + ", a redirect to " + d.redirect.target
}

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
	Codes:        []string{codeArchiveUnreachable, codeArchiveRedirected, codeArchiveInvalid},
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

	configs, err := ExtractConfigs(tree)
	if err != nil {
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     codeArchiveInvalid,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Err(err).Str("; run `ze config validate` on the config and correct the entry it names").String(),
		}}
	}

	var diags []diagnostic.Diagnostic
	for _, ac := range configs {
		if ac.Location == "" {
			continue
		}
		if !strings.HasPrefix(ac.Location, config.ArchiveSchemeHTTP+"://") && !strings.HasPrefix(ac.Location, config.ArchiveSchemeHTTPS+"://") {
			continue
		}
		err := archiveHTTPHead(ac.Location, diagnostic.DoctorProbeTimeout(archiveProbeTimeout))
		if err == nil {
			continue
		}
		if redirected, ok := errors.AsType[*redirectedDestination](err); ok {
			var tb textbuf.Buffer
			diags = append(diags, diagnostic.Diagnostic{
				Code:     codeArchiveRedirected,
				Severity: diagnostic.SeverityWarning,
				Message: tb.Str("archive ").Str(ac.Name).Str(": location ").Str(RedactURL(ac.Location)).Str(" ").Err(err).
					Str("; Ze follows no redirect when it uploads the config, because the upload carries its reversible $9$ secrets, so every upload to this location fails").String(),
				Path: RedactURL(ac.Location),
				Fix:  "set system archive " + ac.Name + " location " + redirected.redirect.target,
			})
			continue
		}
		var tb textbuf.Buffer
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeArchiveUnreachable,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("archive ").Str(ac.Name).Str(": location unreachable: ").Err(err).String(),
			Path:     RedactURL(ac.Location),
		})
	}
	return diags
}

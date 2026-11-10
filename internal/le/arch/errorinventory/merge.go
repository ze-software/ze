// Design: docs/contributing/error-inventory.md -- the inventory of user-visible message sites
//
// Overview: inventory.go -- the row, its surface, status, source and pre-rank
// Related: scan.go -- the writer rows merged here
//
// merge.go turns the two sibling gates' site lists into rows and joins them
// with this package's own writer rows, keyed by file and line, so a site two
// sources name is listed once. It reads each list through the sibling's own
// exported function (archoperatorerror.Sites, archdoctorcoverage.Scan) and
// parses nothing: a second scan of a handler return, a log line or a doctor
// finding would count it twice and drift from the gate that judges it.

package archerrorinventory

import (
	"cmp"
	"errors"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
	archdoctorcoverage "github.com/ze-software/ze/internal/le/arch/doctorcoverage"
	archoperatorerror "github.com/ze-software/ze/internal/le/arch/operatorerror"
)

// Floors are the fewest sites each source must answer for a scan to judge a
// tree. A source below its floor matched a shape that no longer exists (a
// renamed registry, logger or writer helper), and the rows it lost would read
// as a tree with fewer plain messages, which is what a fixed tree answers.
// A floor of zero accepts any count: a fixture tree.
type Floors struct {
	// OperatorErrorSites is the fewest archoperatorerror sites.
	OperatorErrorSites int
	// DoctorCoverageSites is the fewest archdoctorcoverage sites.
	DoctorCoverageSites int
	// DoctorCodes is archdoctorcoverage.Scan's own codesFloor: the fewest
	// doctor codes its covered set must read.
	DoctorCodes int
	// WriterRows is the fewest rows of this package's own writers.
	WriterRows int
}

// ErrThinScan names a scan one of whose sources found fewer sites than the
// tree is known to hold, so the inventory judged too little to answer for it.
var ErrThinScan = errors.New("the error-inventory scan found too few message sites to judge the tree")

// checkFloor refuses a source that answered fewer than floor sites, naming
// the source and what to check.
func checkFloor(source Source, found, floor int) error {
	if found >= floor {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str(": ").Str(source.String()).Str(" answered ").Int(int64(found)).
		Str(" site(s), at least ").Int(int64(floor)).Str(" expected. ")
	switch source {
	case SourceOperatorError:
		tb.Str("Run ./le arch operator-error report: a registration shape in internal/le/arch/operatorerror no longer matches the registry it names")
	case SourceDoctorCoverage:
		tb.Str("Run ./le arch doctor-coverage report: a call shape in internal/le/arch/doctorcoverage/sites.go no longer matches the logger or report bus it names")
	case SourceWriter:
		tb.Str("A row in internal/le/arch/errorinventory/writers.go no longer matches the call it names; update the row to the current call")
	case SourceUnspecified:
		return errors.New("BUG: an error-inventory floor checked for no source")
	}
	return errors.New(ErrThinScan.Error() + tb.String())
}

// errSiteUndeclared refuses a sibling's site whose kind or status no
// constant of this package maps: a row invented for it would place or judge
// it on nothing.
var errSiteUndeclared = errors.New("BUG: an error-inventory source site has a kind or status this inventory does not map")

// siteError names the site errSiteUndeclared refuses.
func siteError(file string, line int, what string) error {
	return errors.New(errSiteUndeclared.Error() + ": " + file + ":" + strconv.Itoa(line) + " " + what)
}

// textStatus judges a message no source proves typed by its literal text
// alone: plain-names-command when the text names a command (namesCommand),
// plain otherwise. It never answers typed.
func textStatus(text string) Status {
	if namesCommand(text) {
		return StatusPlainNamesCommand
	}
	return StatusPlain
}

// operatorErrorRows answers one row per archoperatorerror site. Every site is
// a refusal of a registered handler, ranked by its surface.
func operatorErrorRows(sites []archoperatorerror.Site) ([]Row, error) {
	rows := make([]Row, 0, len(sites))
	for i := range sites {
		row, err := operatorErrorRow(&sites[i])
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// operatorErrorRow answers the row of one handler refusal. Its message is the
// returned expression as source, the only text the site list holds.
func operatorErrorRow(site *archoperatorerror.Site) (Row, error) {
	surface, err := operatorErrorSurface(site)
	if err != nil {
		return Row{}, err
	}
	status, err := operatorErrorStatus(site)
	if err != nil {
		return Row{}, err
	}
	rank, err := preRankOf(surface, SeverityRefusal)
	if err != nil {
		return Row{}, err
	}
	return Row{
		Surface: surface,
		File:    site.File,
		Line:    site.Line,
		EndLine: site.Line,
		Package: dirOf(site.File),
		Status:  status,
		PreRank: rank,
		Source:  SourceOperatorError,
		Writer:  site.Kind.String() + " " + site.Handler,
		Message: site.Expr,
	}, nil
}

// operatorErrorSurface places a handler refusal by its registration kind: a
// config verifier's refusal is config-verify, every command or RPC handler's
// is daemon-command.
func operatorErrorSurface(site *archoperatorerror.Site) (Surface, error) {
	switch site.Kind {
	case archoperatorerror.KindRPC, archoperatorerror.KindStreaming,
		archoperatorerror.KindPluginRPC, archoperatorerror.KindSDKCommand:
		return SurfaceDaemonCommand, nil
	case archoperatorerror.KindConfigVerifier, archoperatorerror.KindSDKConfigVerify:
		return SurfaceConfigVerify, nil
	case archoperatorerror.KindUnspecified:
		return SurfaceUnspecified, siteError(site.File, site.Line, "kind "+site.Kind.String())
	}
	return SurfaceUnspecified, siteError(site.File, site.Line, "kind "+site.Kind.String())
}

// operatorErrorStatus maps operator-error's status. Typed stays typed. A
// pass-through hands on an error value the site did not build, so nothing
// proves it carries a fix: it is judged like a plain construction, by the
// literal text of its expression, and never reads as typed.
func operatorErrorStatus(site *archoperatorerror.Site) (Status, error) {
	switch site.Status {
	case archoperatorerror.StatusTyped:
		return StatusTyped, nil
	case archoperatorerror.StatusPlain, archoperatorerror.StatusPassThrough:
		return textStatus(site.Expr), nil
	case archoperatorerror.StatusUnspecified:
		return StatusUnspecified, siteError(site.File, site.Line, "status "+site.Status.String())
	}
	return StatusUnspecified, siteError(site.File, site.Line, "status "+site.Status.String())
}

// doctorCoverageRows answers one row per archdoctorcoverage site.
func doctorCoverageRows(sites []archdoctorcoverage.Site) ([]Row, error) {
	rows := make([]Row, 0, len(sites))
	for i := range sites {
		row, err := doctorCoverageRow(&sites[i])
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// doctorCoverageRow answers the row of one log line, report raise, health
// row or doctor finding, ranked by the level its kind declares.
func doctorCoverageRow(site *archdoctorcoverage.Site) (Row, error) {
	surface, severity, err := doctorCoveragePlace(site)
	if err != nil {
		return Row{}, err
	}
	status, err := doctorCoverageStatus(site)
	if err != nil {
		return Row{}, err
	}
	rank, err := preRankOf(surface, severity)
	if err != nil {
		return Row{}, err
	}
	return Row{
		Surface: surface,
		File:    site.File,
		Line:    site.Line,
		EndLine: max(site.EndLine, site.Line),
		Package: site.Package,
		Status:  status,
		PreRank: rank,
		Source:  SourceDoctorCoverage,
		Writer:  site.Call,
		Message: site.Message,
	}, nil
}

// doctorCoveragePlace answers a site's surface and severity from its kind. A
// log line and a report raise name their level. A health row and a doctor
// finding name none the site list carries (a finding's Severity field is not
// in archdoctorcoverage.Site), and both are a problem the operator is shown
// as one, so they rank as errors: ranking an unread level below the warnings
// could sink an error to the end of the worklist.
func doctorCoveragePlace(site *archdoctorcoverage.Site) (Surface, Severity, error) {
	switch site.Kind {
	case archdoctorcoverage.KindLogWarn, archdoctorcoverage.KindReportWarning:
		return SurfaceLog, SeverityWarning, nil
	case archdoctorcoverage.KindLogError, archdoctorcoverage.KindReportError,
		archdoctorcoverage.KindHealthRow:
		return SurfaceLog, SeverityError, nil
	case archdoctorcoverage.KindDoctorFinding:
		return SurfaceDoctor, SeverityError, nil
	case archdoctorcoverage.KindUnspecified:
		return SurfaceUnspecified, SeverityUnspecified, siteError(site.File, site.Line, "kind "+site.Kind.String())
	}
	return SurfaceUnspecified, SeverityUnspecified, siteError(site.File, site.Line, "kind "+site.Kind.String())
}

// doctorCoverageStatus maps doctor-coverage's status. A doctor finding whose
// Fix is set is typed: the Diagnostic record is the doctor surface's operator
// error type, and its Fix is the command that clears it. Every other site is
// judged by its message's text. A log line or report raise that names a
// declared doctor code is covered for doctor's purposes, which proves ze
// doctor reports the problem, not that the line itself names a fix: its
// "fix" attribute, where one exists, is not in the site list, so it is not
// read here either.
func doctorCoverageStatus(site *archdoctorcoverage.Site) (Status, error) {
	switch site.Status {
	case archdoctorcoverage.StatusFixSet:
		return StatusTyped, nil
	case archdoctorcoverage.StatusNoFix, archdoctorcoverage.StatusCovered,
		archdoctorcoverage.StatusNoCode, archdoctorcoverage.StatusCodeDynamic,
		archdoctorcoverage.StatusCodeNotDoctor, archdoctorcoverage.StatusHealthRow:
		return textStatus(site.Message), nil
	case archdoctorcoverage.StatusUnspecified:
		return StatusUnspecified, siteError(site.File, site.Line, "status "+site.Status.String())
	}
	return StatusUnspecified, siteError(site.File, site.Line, "status "+site.Status.String())
}

// siteKey is the identity the merge keys a row by.
type siteKey struct {
	file string
	line int
}

// mergeRows answers rows with one row per file and line. Where two rows
// share one, the row of the earlier Source wins (operator-error, then
// doctor-coverage, then this package's writers), whatever order rows
// arrived in; within one source the first row wins. The answer is in source
// order; the report sorts it for reading.
func mergeRows(rows []Row) []Row {
	ordered := slices.Clone(rows)
	slices.SortStableFunc(ordered, func(a, b Row) int { return cmp.Compare(a.Source, b.Source) })
	seen := make(map[siteKey]bool, len(ordered))
	merged := make([]Row, 0, len(ordered))
	for i := range ordered {
		key := siteKey{file: ordered[i].File, line: ordered[i].Line}
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, ordered[i])
	}
	return merged
}

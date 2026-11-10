// Design: docs/contributing/error-inventory.md -- the inventory of user-visible message sites
//
// Detail: report.go -- what check and report answer
// Related: actions.go -- the command surface
//
// inventory.go declares what one message site IS: the surface a user meets it
// on, where it sits, who owns it, whether it meets
// ai/rules/user-facing-errors.md, and how early it belongs on the worklist.
// The status is read from the code, never judged: only the operator error
// type carrying a fix meets the rule.
//
// The rows come from three sources. Handler returns and error answers are
// archoperatorerror's sites, log lines, report raises, health rows and doctor
// findings are archdoctorcoverage's, and only the surface writers neither of
// them covers are found here, so no surface is scanned twice (merge.go).

package archerrorinventory

import (
	"errors"
	"os"
	"slices"

	archdoctorcoverage "github.com/ze-software/ze/internal/le/arch/doctorcoverage"
	archoperatorerror "github.com/ze-software/ze/internal/le/arch/operatorerror"
	"github.com/ze-software/ze/internal/le/hookruntime"
)

// The names String answers for a value no constant declares.
const (
	nameUnspecified = "unspecified"
	nameUnknown     = "unknown"
)

// Surface is where a user meets a message.
type Surface uint8

// The surfaces. The zero value is no surface, so a Row nobody placed cannot
// read as one. The order is the order of the count lines.
const (
	SurfaceUnspecified Surface = iota
	// SurfaceDaemonCommand is a registered command or RPC handler's answer,
	// served on the CLI, SSH, MCP and REST alike.
	SurfaceDaemonCommand
	// SurfaceConfigVerify is a registered config verifier's refusal.
	SurfaceConfigVerify
	// SurfaceLog is a Warn or Error log line, a report-bus raise or a health
	// row.
	SurfaceLog
	// SurfaceDoctor is a diagnostic.Diagnostic finding: ze doctor, config
	// validation and ze config fix --plan.
	SurfaceDoctor
	// SurfaceLocalCLI is the local ze program's own refusal or stderr write.
	SurfaceLocalCLI
	// SurfaceMCP is an MCP tool's error result.
	SurfaceMCP
	// SurfaceREST is a REST error body.
	SurfaceREST
	// SurfaceWeb is a web UI refusal.
	SurfaceWeb
	// SurfaceGNMI is a gNMI status error.
	SurfaceGNMI
	// SurfaceGRPC is a status error of the gRPC command API, which shares
	// gNMI's writer and is a surface of its own.
	SurfaceGRPC
	// SurfaceTooling is an ./le refusal or finding.
	SurfaceTooling
)

// surfaces is every surface a row can name, in count-line order.
var surfaces = []Surface{
	SurfaceDaemonCommand, SurfaceConfigVerify, SurfaceLog, SurfaceDoctor,
	SurfaceLocalCLI, SurfaceMCP, SurfaceREST, SurfaceWeb, SurfaceGNMI, SurfaceGRPC,
	SurfaceTooling,
}

// errSurfaceUnspecified refuses to render a row nobody placed.
var errSurfaceUnspecified = errors.New("BUG: an error-inventory row has no surface")

// String renders the surface by name.
func (s Surface) String() string {
	switch s {
	case SurfaceDaemonCommand:
		return "daemon-command"
	case SurfaceConfigVerify:
		return "config-verify"
	case SurfaceLog:
		return "log"
	case SurfaceDoctor:
		return "doctor"
	case SurfaceLocalCLI:
		return "local-cli"
	case SurfaceMCP:
		return "mcp"
	case SurfaceREST:
		return "rest"
	case SurfaceWeb:
		return "web"
	case SurfaceGNMI:
		return "gnmi"
	case SurfaceGRPC:
		return "grpc"
	case SurfaceTooling:
		return "le"
	case SurfaceUnspecified:
		return nameUnspecified
	}
	return nameUnknown
}

// MarshalText renders the surface by name in JSON, and refuses a row nobody
// placed rather than print it as a surface.
func (s Surface) MarshalText() ([]byte, error) {
	if s == SurfaceUnspecified {
		return nil, errSurfaceUnspecified
	}
	return []byte(s.String()), nil
}

// Status is what one site answers about the message rule.
type Status uint8

// The statuses. The zero value is no status, so a Row nobody judged cannot
// read as typed.
const (
	StatusUnspecified Status = iota
	// StatusTyped hands the user the operator error type carrying a fix: the
	// only status that meets the rule.
	StatusTyped
	// StatusPlainNamesCommand is a plain message whose literal text names a
	// ze, ./le or config command. It sorts the worklist and never meets the
	// rule: no tool can prove the command fixes the problem.
	StatusPlainNamesCommand
	// StatusPlain is any other message.
	StatusPlain
)

// errStatusUnspecified refuses to render a row nobody judged.
var errStatusUnspecified = errors.New("BUG: an error-inventory row has no status")

// String renders the status by name.
func (s Status) String() string {
	switch s {
	case StatusTyped:
		return "typed"
	case StatusPlainNamesCommand:
		return "plain-names-command"
	case StatusPlain:
		return "plain"
	case StatusUnspecified:
		return nameUnspecified
	}
	return nameUnknown
}

// MarshalText renders the status by name in JSON, and refuses a row nobody
// judged rather than print it as a status.
func (s Status) MarshalText() ([]byte, error) {
	if s == StatusUnspecified {
		return nil, errStatusUnspecified
	}
	return []byte(s.String()), nil
}

// Source is the package whose site list a row came from. The order is the
// merge precedence: a file and line two sources name keeps the earlier
// source's row.
type Source uint8

// The sources, in merge precedence. The zero value is no source.
const (
	SourceUnspecified Source = iota
	// SourceOperatorError is archoperatorerror.Sites.
	SourceOperatorError
	// SourceDoctorCoverage is archdoctorcoverage.Scan.
	SourceDoctorCoverage
	// SourceWriter is this package's own surface writers, the only rows the
	// check refuses.
	SourceWriter
)

// String renders the source by name.
func (s Source) String() string {
	switch s {
	case SourceOperatorError:
		return "operator-error"
	case SourceDoctorCoverage:
		return "doctor-coverage"
	case SourceWriter:
		return "writer"
	case SourceUnspecified:
		return nameUnspecified
	}
	return nameUnknown
}

// MarshalText renders the source by name in JSON.
func (s Source) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// PreRank orders the worklist from declared facts, never from a judgement of
// harm: 1 comes first.
type PreRank uint8

// The pre-ranks. The zero value is no rank, and a rank above
// PreRankTooling is a defect, so neither renders.
const (
	PreRankUnspecified PreRank = iota
	// PreRankOperatorError is an operator surface with error severity, or a
	// refusal.
	PreRankOperatorError
	// PreRankOperatorWarning is an operator surface with warning severity.
	PreRankOperatorWarning
	// PreRankTooling is ./le tooling.
	PreRankTooling
)

// errPreRankOutOfRange refuses to render a rank outside 1..3.
var errPreRankOutOfRange = errors.New("BUG: an error-inventory row has a pre-rank outside 1..3")

// Valid reports whether the rank is one of the three declared ranks.
func (r PreRank) Valid() bool {
	return r >= PreRankOperatorError && r <= PreRankTooling
}

// MarshalJSON renders the rank as its number, and refuses one outside 1..3.
func (r PreRank) MarshalJSON() ([]byte, error) {
	if !r.Valid() {
		return nil, errPreRankOutOfRange
	}
	return []byte{'0' + byte(r)}, nil
}

// Row is one place a message reaches a user.
type Row struct {
	Surface Surface `json:"surface"`
	// File is slash-separated and relative to the checkout root.
	File    string `json:"file"`
	Line    int    `json:"line"`
	EndLine int    `json:"end-line"`
	// Package is the owner package's directory, relative to the root.
	Package string `json:"package"`
	// Doors names the door pages whose base covers the file.
	Doors   []string `json:"doors"`
	Status  Status   `json:"status"`
	PreRank PreRank  `json:"pre-rank"`
	Source  Source   `json:"source"`
	// Writer is the call that hands the message to the user, as written.
	Writer string `json:"writer"`
	// Message is the message's literal text where the source holds one.
	Message string `json:"message"`
}

// Inventory is every row the scan found.
type Inventory struct {
	Rows []Row
}

// Severity is how a site's own source declares its message: a refusal, an
// error or a warning. It is read, never judged: a writer's answer is a
// refusal, and a log line or report raise names its level.
type Severity uint8

// The severities. The zero value is no severity, so a site nobody read
// cannot rank.
const (
	SeverityUnspecified Severity = iota
	// SeverityRefusal is an answer that refuses what the user asked: every
	// surface writer's row.
	SeverityRefusal
	// SeverityError is an error-level log line, report raise or finding.
	SeverityError
	// SeverityWarning is a warning-level one.
	SeverityWarning
)

// String renders the severity by name.
func (s Severity) String() string {
	switch s {
	case SeverityRefusal:
		return "refusal"
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityUnspecified:
		return nameUnspecified
	}
	return nameUnknown
}

// errPreRankUndeclared refuses to rank a site whose surface or severity no
// constant declares: a rank invented for it would sort the worklist on
// nothing.
var errPreRankUndeclared = errors.New("BUG: an error-inventory site has no declared surface or severity to rank by")

// preRankOf answers a site's pre-rank from its surface and severity: ./le
// tooling is PreRankTooling whatever its severity, and an operator surface is
// PreRankOperatorError for a refusal or an error and PreRankOperatorWarning
// for a warning. A surface or severity no constant declares is refused, so no
// rank outside 1..3 is ever answered.
func preRankOf(surface Surface, severity Severity) (PreRank, error) {
	switch severity {
	case SeverityRefusal, SeverityError, SeverityWarning:
	case SeverityUnspecified:
		return PreRankUnspecified, errPreRankUndeclared
	default:
		return PreRankUnspecified, errPreRankUndeclared
	}
	switch surface {
	case SurfaceTooling:
		return PreRankTooling, nil
	case SurfaceDaemonCommand, SurfaceConfigVerify, SurfaceLog, SurfaceDoctor,
		SurfaceLocalCLI, SurfaceMCP, SurfaceREST, SurfaceWeb, SurfaceGNMI, SurfaceGRPC:
		if severity == SeverityWarning {
			return PreRankOperatorWarning, nil
		}
		return PreRankOperatorError, nil
	case SurfaceUnspecified:
		return PreRankUnspecified, errPreRankUndeclared
	}
	return PreRankUnspecified, errPreRankUndeclared
}

// errNotATree refuses a root that is not a directory.
var errNotATree = errors.New("the error-inventory root is not a directory")

// Scan answers the inventory of the tree at root: archoperatorerror's
// handler refusals, archdoctorcoverage's log lines, report raises, health
// rows and doctor findings, and this package's own writer rows (writerRows),
// merged so each file and line is one row (mergeRows), each row carrying the
// door pages that cover its file (hookruntime.DoorsCovering, the lookup
// doctor-coverage reads too). A source that answers fewer sites than its
// floor is an error (checkFloor), and so is a root that is not a directory:
// a scan that could not look is never an empty inventory.
func Scan(root string, floors Floors) (Inventory, error) {
	info, err := os.Stat(root)
	if err != nil {
		return Inventory{}, err
	}
	if !info.IsDir() {
		return Inventory{}, errNotATree
	}
	doorsOf, err := hookruntime.DoorsCovering(root)
	if err != nil {
		return Inventory{}, err
	}
	operatorSites, err := archoperatorerror.Sites(root)
	if err != nil {
		return Inventory{}, err
	}
	if err := checkFloor(SourceOperatorError, len(operatorSites), floors.OperatorErrorSites); err != nil {
		return Inventory{}, err
	}
	doctor, err := archdoctorcoverage.Scan(root, floors.DoctorCodes, doorsOf)
	if err != nil {
		return Inventory{}, err
	}
	if err := checkFloor(SourceDoctorCoverage, len(doctor.Sites), floors.DoctorCoverageSites); err != nil {
		return Inventory{}, err
	}
	written, err := writerRows(root, nil)
	if err != nil {
		return Inventory{}, err
	}
	if err := checkFloor(SourceWriter, len(written), floors.WriterRows); err != nil {
		return Inventory{}, err
	}
	operatorRows, err := operatorErrorRows(operatorSites)
	if err != nil {
		return Inventory{}, err
	}
	doctorRows, err := doctorCoverageRows(doctor.Sites)
	if err != nil {
		return Inventory{}, err
	}
	rows := mergeRows(slices.Concat(operatorRows, doctorRows, written))
	for i := range rows {
		rows[i].Doors = append([]string{}, doorsOf(rows[i].File)...)
	}
	return Inventory{Rows: rows}, nil
}

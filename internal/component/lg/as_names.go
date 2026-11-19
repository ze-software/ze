// Design: docs/architecture/web-interface.md -- LG AS names: asked of the resolve component through the dispatcher
// Overview: server.go -- the looking glass server whose dispatcher answers the names
// Related: handler_graph.go -- the graphs that label their AS nodes
// Related: handler_ui.go -- the peer tables and the peer detail

package lg

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/report"
)

// asNameCommand is the command whose answer names an AS number: the resolve
// component's `resolve cymru asn-name <asn>` (wire method
// ze-resolve:cymru-asn-name, handleCymruASNName in
// internal/component/resolve/cmd/resolve.go). The looking glass asks the
// dispatcher it already holds, as it asks for every other fact, so it holds
// no resolver and imports no resolve package. The resolve component answers
// from its Team Cymru client, which caches each name for an hour.
const asNameCommand = "resolve cymru asn-name "

// ASNameUnknown is the name the command answers for an AS number Team Cymru
// names nothing for: an answer, drawn as the AS number alone. It is a copy of
// internal/component/resolve/cmd ASNameUnknown, which this package cannot
// import; cmd/ze/hub/service_lg_test.go compares the two.
const ASNameUnknown = "(unknown)"

// asNameBudget bounds the name lookups of one page part: one graph, one peer
// table, one peer's detail. The lookups share one deadline, and a lookup
// still running when it passes ends then, because the deadline reaches the
// DNS exchange (dns.Resolver.ResolveTXTWithin). A lookup asked after it has
// passed is not dispatched. So a graph of maxGraphNodes AS numbers that are
// not cached yet waits for their names at most this long, and draws the
// numbers it could not name alone.
const asNameBudget = 2 * time.Second

// asNameDoctorCode is the doctor code a failed lookup is logged and raised
// under. A lookup fails when the router's name servers do not answer, which
// the system component's DNS check (codeDNSResolver,
// internal/component/config/system/doctor.go) diagnoses.
const asNameDoctorCode = "doctor-dns-resolver"

// The report-bus identity of the failure: one warning for the whole looking
// glass, refreshed by each page part that fails and cleared by the next one
// that names an AS number with no failure.
const (
	asNameReportSource  = "lg"
	asNameReportSubject = "as-names"
)

// errNoNameInAnswer is the failure of an answer that carries no name string.
var errNoNameInAnswer = errors.New("the answer carries no name")

// asNames names the AS numbers of one page part. Get one from
// LGServer.asNames, ask with name, and MUST call done when the part is drawn:
// done ends the deadline and reports the failures.
//
// Not safe for concurrent use: the handler drawing the page part owns it.
type asNames struct {
	dispatch CommandDispatcher
	logger   *slog.Logger
	ctx      context.Context
	cancel   context.CancelFunc

	// known holds the answer for each AS number this page part asked for,
	// so a graph that labels a node twice (its layout, then its drawing)
	// asks once. It lives as long as the page part, and holds at most its
	// AS numbers: maxGraphNodes for a graph.
	known map[uint32]string

	// named counts the lookups that answered, a name or ASNameUnknown.
	named int
	// failed counts the lookups that could not learn a name; firstFailed and
	// firstErr are the first of them, which done reports.
	failed      int
	firstFailed uint32
	firstErr    error
}

// asNames starts the name lookups of one page part. The caller MUST call done.
func (s *LGServer) asNames() *asNames {
	ctx, cancel := context.WithTimeout(context.Background(), asNameBudget)
	return &asNames{
		dispatch: s.dispatch,
		logger:   s.logger,
		ctx:      ctx,
		cancel:   cancel,
		known:    make(map[uint32]string),
	}
}

// name answers the organization name of the AS number text, or "" when it has
// none to show: the AS has no name, the lookup failed (counted for done), or
// text names no AS number.
//
// The text arrives as the daemon rendered it, so under bgp/as-notation asdot
// it reads "1.10". It is parsed to the AS number here, and the command is
// asked in plain decimal, so one AS is one lookup whatever its spelling.
func (n *asNames) name(text string) string {
	if text == "" {
		return ""
	}
	number, err := asn.Parse(text)
	if err != nil {
		// A token that names no AS number has no name to ask for.
		return ""
	}
	if name, asked := n.known[number]; asked {
		return name
	}
	name := n.ask(number)
	n.known[number] = name
	return name
}

// ask dispatches the name command for one AS number.
func (n *asNames) ask(number uint32) string {
	// A server built without a dispatcher names nothing. NewLGServer refuses
	// one, so only a unit test's bare LGServer reaches this guard.
	if n.dispatch == nil {
		return ""
	}
	if err := n.ctx.Err(); err != nil {
		n.fail(number, fmt.Errorf("not asked, the %s budget for the page was spent: %w", asNameBudget, err))
		return ""
	}

	command := asNameCommand + asn.Text(number, asn.NotationPlain)
	result, err := n.dispatch.JSON(n.ctx, plugin.CallerIdentity{}, command)
	defer result.TransportComplete()
	if err != nil {
		n.fail(number, err)
		return ""
	}

	answer := parseJSON(result.Output)
	name, isString := answer["name"].(string)
	if !isString {
		n.fail(number, errNoNameInAnswer)
		return ""
	}
	n.named++
	if name == ASNameUnknown {
		return ""
	}
	return name
}

// fail counts a lookup that learned no name, and keeps the first for done.
func (n *asNames) fail(number uint32, err error) {
	if n.failed == 0 {
		n.firstFailed = number
		n.firstErr = err
	}
	n.failed++
}

// done ends the deadline and reports the page part's failures: one WARN line
// and one report-bus warning for the part, however many lookups failed, so a
// graph of a hundred AS numbers with the name servers down logs once per
// drawing rather than a hundred times. A part that named at least one AS
// number and failed none clears the warning.
func (n *asNames) done() {
	n.cancel()
	if n.failed == 0 {
		if n.named > 0 {
			report.ClearWarning(asNameReportSource, asNameDoctorCode, asNameReportSubject)
		}
		return
	}

	// The fix shows the whole reason for the first AS number that failed.
	fix := asNameCommand + asn.Text(n.firstFailed, asn.NotationPlain)
	n.logger.Warn("looking glass shows AS numbers without their names: the AS name lookup failed",
		"code", asNameDoctorCode,
		"unnamed", n.failed,
		"first-as", n.firstFailed,
		"error", n.firstErr.Error(),
		"fix", fix)

	message := fmt.Sprintf("the looking glass could not name %d AS numbers and shows them as numbers alone: AS%d: %v",
		n.failed, n.firstFailed, n.firstErr)
	report.RaiseWarningFix(asNameReportSource, asNameDoctorCode, asNameReportSubject, report.FitMessage(message), fix)
}

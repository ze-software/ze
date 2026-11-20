// Design: docs/architecture/api/commands.md — local-data commands, text and bytes answers
// Related: register.go — the registrations these handlers serve
//
// thanks.go answers `ze thanks` with one row per third-party work this binary
// carries, and `ze thanks licence` with the licence document, narrowed to one
// work by the `name` pipe operation. Both read internal/core/credits through
// creditsSource, so a unit test answers from a fixture instead of the
// generated data.

package thanks

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/credits"
	"github.com/ze-software/ze/internal/core/opererr"
)

// The JSON keys of one row, which are also the declared columns, in the order
// the list prints them.
const (
	keyGroup     = "group"
	keyName      = "name"
	keyVersion   = "version"
	keyLicence   = "licence"
	keyCopyright = "copyright"
	keyWhy       = "why"
	keySource    = "source"
)

// keyWorks is the payload key the list carries its rows under.
const keyWorks = "works"

// operationName is the pipe operation that selects one work by its exact name.
const operationName = "name"

// commandList is the daemon form of `ze thanks`, which every refusal about a
// work's name offers as the way to see the names.
const commandList = "show thanks list"

// creditsSource is the credits record the two handlers answer from.
// compiledCredits is the one production implementation.
type creditsSource interface {
	// Works answers one record per carried work, or the reason it cannot.
	Works() ([]credits.Work, error)
	// LicenceDocument answers the licence document of every carried work.
	LicenceDocument() ([]byte, error)
	// LicenceOf answers the licence document of the one work named name, or an
	// error matching credits.ErrUnknownWork when no carried work has that name.
	LicenceOf(name string) ([]byte, error)
}

// compiledCredits answers from internal/core/credits, the record compiled into
// this binary. It holds nothing, so it is safe for concurrent use.
type compiledCredits struct{}

func (compiledCredits) Works() ([]credits.Work, error)        { return credits.Works() }
func (compiledCredits) LicenceDocument() ([]byte, error)      { return credits.LicenceDocument() }
func (compiledCredits) LicenceOf(name string) ([]byte, error) { return credits.LicenceOf(name) }

// workRow is one credited work as the list answers it.
type workRow struct {
	Group     string `json:"group"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Licence   string `json:"licence"`
	Copyright string `json:"copyright"`
	Why       string `json:"why"`
	Source    string `json:"source"`
}

// listHandler answers `ze thanks` and `show thanks list` from source: one row
// per carried work, or a refusal saying why there is no list.
func listHandler(source creditsSource) command.LocalDataHandler {
	return func(command.ValidatedArgs) command.LocalAnswer {
		works, err := source.Works()
		if err != nil {
			return creditsRefusal("ze thanks", err)
		}
		// Every binary links Go's own library, so a record that answers no
		// work is a defect of the record, never a binary that carries none.
		if len(works) == 0 {
			return command.LocalRefusal(1, command.LocalErrorLine(opererr.New(
				"ze thanks found no work in this binary's credits record",
				"every binary carries at least Go's own library, so the build that made this binary is defective and must be redone",
				"./le deps credits check")))
		}
		rows := make([]workRow, 0, len(works))
		for _, work := range works {
			rows = append(rows, workRow{
				Group:     work.Group,
				Name:      work.Name,
				Version:   work.Version,
				Licence:   work.Licence,
				Copyright: work.Copyright,
				Why:       work.Why,
				Source:    work.Source,
			})
		}
		return command.LocalAnswer{Payload: map[string]any{keyWorks: rows}}
	}
}

// licenceHandler answers `ze thanks licence` and `show thanks licence` from
// source: the whole licence document, or one work's under `| name <work>`.
func licenceHandler(source creditsSource) command.LocalDataHandler {
	return func(args command.ValidatedArgs) command.LocalAnswer {
		// The declared `name` operation is the only one the command declares,
		// so every operation in the list is a `name`.
		operations := args.PipeOperations()
		if len(operations) == 0 {
			document, err := source.LicenceDocument()
			if err != nil {
				return creditsRefusal("ze thanks licence", err)
			}
			return command.LocalAnswer{Payload: command.Blob(document)}
		}
		if len(operations) > 1 {
			first := strings.Join(operations[0].Args, " ")
			return command.LocalRefusal(1, command.LocalErrorLine(opererr.New(
				"the name operation is given more than once",
				"the licence document narrows to one work at a time",
				"show thanks licence | name "+first)))
		}
		if len(operations[0].Args) == 0 {
			return command.LocalRefusal(1, command.LocalErrorLine(opererr.New(
				"the name operation names no work",
				"it selects one work by its exact name, as the list prints it",
				commandList)))
		}
		// The argument words are joined with one space, because a work that
		// is not a Go module has a name of several words.
		name := strings.Join(operations[0].Args, " ")
		document, err := source.LicenceOf(name)
		if errors.Is(err, credits.ErrUnknownWork) {
			return command.LocalRefusal(1, command.LocalErrorLine(opererr.Wrap(err,
				"no work named "+name+" is carried by this binary",
				"the name operation selects one work by its exact name, as the list prints it",
				commandList)))
		}
		if err != nil {
			return creditsRefusal("ze thanks licence", err)
		}
		return command.LocalAnswer{Payload: command.Blob(document)}
	}
}

// creditsRefusal answers the refusal of a command whose credits record cannot
// answer. No operator action on the box repairs it: the record is compiled in
// at build time, so the binary came from a build that skipped generating it,
// and whoever builds Ze fixes it by generating the record and building again.
func creditsRefusal(invocation string, cause error) command.LocalAnswer {
	return command.LocalRefusal(1, command.LocalErrorLine(opererr.Wrap(cause,
		invocation+" cannot list the third-party works this binary carries",
		"this binary was built without its credits record, so it must be rebuilt from the Ze repository after the record is generated",
		"./le deps credits update")))
}

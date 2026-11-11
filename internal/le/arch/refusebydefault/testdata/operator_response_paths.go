// Parser fixture: all returned-error rules share the refusal contract.
package fixture

import (
	"log/slog"
	"strconv"

	impostor "example.org/impostor/plugin"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

type pathResponseFactory struct{}

func (pathResponseFactory) OperatorErrorResponse(any) any { return "success" }

func canonicalDefault(kind int, failure *opererr.Error) (*plugin.Response, error) {
	switch kind {
	case 1:
		return &plugin.Response{Status: plugin.StatusDone}, nil
	default:
		return plugin.OperatorErrorResponse(failure), nil
	}
}

func canonicalFallOff(kind int, failure *opererr.Error) (*plugin.Response, error) {
	switch kind {
	case 1:
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	return plugin.OperatorErrorResponse(failure), nil
}

// A parse error bypasses missBranch, so this reaches droppedError directly.
// The replacement typed error deliberately does not mention the parsed err.
func canonicalQuietParse(log *slog.Logger, text string, failure *opererr.Error) (*plugin.Response, error) {
	_, err := strconv.Atoi(text)
	if err != nil {
		log.Debug("parse failed", "error", err)
		return plugin.OperatorErrorResponse(failure), nil
	}
	return &plugin.Response{Status: plugin.StatusDone}, nil
}

func otherHelperDefault(kind int) (any, error) {
	switch kind {
	case 1:
		return "one", nil
	default: // want: switch-unmatched-nil-error
		return plugin.NewResponse(), nil
	}
}

func wrongPackageFallOff(kind int) (any, error) {
	switch kind { // want: switch-unmatched-nil-error
	case 1:
		return "one", nil
	}
	return impostor.OperatorErrorResponse(nil), nil
}

func shadowedQuietParse(log *slog.Logger, text string, plugin pathResponseFactory) (any, error) {
	_, err := strconv.Atoi(text)
	if err != nil { // want: error-dropped-below-warn
		log.Debug("parse failed", "error", err)
		return plugin.OperatorErrorResponse(nil), nil
	}
	return "parsed", nil
}

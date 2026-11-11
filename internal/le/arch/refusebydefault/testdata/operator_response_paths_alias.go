// Parser fixture: import aliases carry the same identity on every rule path.
package fixture

import (
	"log/slog"
	"strconv"

	impostor "example.org/impostor/plugin"
	sdk "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

type aliasedPathResponseFactory struct{}

func (aliasedPathResponseFactory) OperatorErrorResponse(any) any { return "success" }

func aliasedDefault(kind int, failure *opererr.Error) (*sdk.Response, error) {
	switch kind {
	case 1:
		return &sdk.Response{Status: sdk.StatusDone}, nil
	default:
		return sdk.OperatorErrorResponse(failure), nil
	}
}

func aliasedFallOff(kind int, failure *opererr.Error) (*sdk.Response, error) {
	switch kind {
	case 1:
		return &sdk.Response{Status: sdk.StatusDone}, nil
	}
	return sdk.OperatorErrorResponse(failure), nil
}

func aliasedQuietParse(log *slog.Logger, text string, failure *opererr.Error) (*sdk.Response, error) {
	_, err := strconv.Atoi(text)
	if err == nil {
		return &sdk.Response{Status: sdk.StatusDone}, nil
	} else {
		log.Debug("parse failed", "error", err)
		return sdk.OperatorErrorResponse(failure), nil
	}
}

func shadowedDefault(kind int, sdk aliasedPathResponseFactory) (any, error) {
	switch kind {
	case 1:
		return "one", nil
	default: // want: switch-unmatched-nil-error
		return sdk.OperatorErrorResponse(nil), nil
	}
}

func otherHelperFallOff(kind int) (any, error) {
	switch kind { // want: switch-unmatched-nil-error
	case 1:
		return "one", nil
	}
	return sdk.NewResponse(), nil
}

func wrongPackageQuietParse(log *slog.Logger, text string) (any, error) {
	_, err := strconv.Atoi(text)
	if err != nil { // want: error-dropped-below-warn
		log.Debug("parse failed", "error", err)
		return impostor.OperatorErrorResponse(nil), nil
	}
	return "parsed", nil
}

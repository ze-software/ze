// Parser fixture: the import path establishes identity, and locals hide it.
package fixture

import (
	sdk "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

type responseFactory struct{}

func (responseFactory) OperatorErrorResponse(any) any { return "success" }

func aliasedResponse(value any, failure *opererr.Error) (*sdk.Response, error) {
	text, ok := value.(string)
	if !ok {
		return sdk.OperatorErrorResponse(failure), nil
	}
	return &sdk.Response{Data: text}, nil
}

func shadowedParameter(value any, sdk responseFactory) (any, error) {
	text, ok := value.(string)
	if !ok { // want: miss-answered-as-value
		return sdk.OperatorErrorResponse(nil), nil
	}
	return text, nil
}

func shadowedLocal(value any) (any, error) {
	sdk := responseFactory{}
	text, ok := value.(string)
	if !ok { // want: miss-answered-as-value
		return sdk.OperatorErrorResponse(nil), nil
	}
	return text, nil
}

func shadowedCapture(value any) func() (any, error) {
	sdk := responseFactory{}
	return func() (any, error) {
		text, ok := value.(string)
		if !ok { // want: miss-answered-as-value
			return sdk.OperatorErrorResponse(nil), nil
		}
		return text, nil
	}
}

func restoredImportScope(value any, failure *opererr.Error) (*sdk.Response, error) {
	{
		sdk := responseFactory{}
		_ = sdk
	}
	text, ok := value.(string)
	if !ok {
		return sdk.OperatorErrorResponse(failure), nil
	}
	return &sdk.Response{Data: text}, nil
}

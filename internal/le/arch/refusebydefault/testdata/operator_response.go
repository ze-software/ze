// Parser fixture: canonical error responses carry the miss to the caller.
package fixture

import (
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

func canonicalResponse(value any, failure *opererr.Error) (*plugin.Response, error) {
	text, ok := value.(string)
	if !ok {
		return plugin.OperatorErrorResponse(failure), nil
	}
	return &plugin.Response{Data: text}, nil
}

func parenthesizedResponse(value any, failure *opererr.Error) (*plugin.Response, error) {
	text, ok := value.(string)
	if !ok {
		return (plugin.OperatorErrorResponse(failure)), nil
	}
	return &plugin.Response{Data: text}, nil
}

func successResponse(value any) (*plugin.Response, error) {
	text, ok := value.(string)
	if !ok { // want: miss-answered-as-value
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	return &plugin.Response{Data: text}, nil
}

func otherCanonicalPackageHelper(value any) (*plugin.Response, error) {
	text, ok := value.(string)
	if !ok { // want: miss-answered-as-value
		return plugin.NewResponse(), nil
	}
	return &plugin.Response{Data: text}, nil
}

func wrappedCanonicalResponse(value any, failure *opererr.Error, success func(*plugin.Response) *plugin.Response) (*plugin.Response, error) {
	text, ok := value.(string)
	if !ok { // want: miss-answered-as-value
		return success(plugin.OperatorErrorResponse(failure)), nil
	}
	return &plugin.Response{Data: text}, nil
}

func localSameSpelling(value any, OperatorErrorResponse func() any) (any, error) {
	text, ok := value.(string)
	if !ok { // want: miss-answered-as-value
		return OperatorErrorResponse(), nil
	}
	return text, nil
}

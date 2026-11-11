// Parser fixture: the same import alias and helper name in another package
// do not establish the canonical operator-error response contract.
package fixture

import plugin "example.org/impostor/plugin"

func differentPackage(value any) (any, error) {
	text, ok := value.(string)
	if !ok { // want: miss-answered-as-value
		return plugin.OperatorErrorResponse(nil), nil
	}
	return text, nil
}

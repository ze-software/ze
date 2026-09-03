// VALIDATES: the ze-radius-conf.yang system/authentication/radius/auth-method
// enum accepts the two credentials the RADIUS admin authenticator can build,
// and refuses anything else at config load with the leaf path and the permitted
// values. The Go side of the same boundary is parseAuthMethod
// (internal/component/radius/config.go), tested by TestExtractConfigAuthMethod.
// PREVENTS: schema-and-runtime drift on the credential selector. A word the
// schema accepts and the runtime does not disables the RADIUS backend at build
// time and drops every operator login to the local backend; a word the schema
// refuses and the runtime supports makes a documented value unusable.
package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	// Register ze-radius-conf so ValidateTreeAllModules finds the system
	// section it contributes (the live binary registers it via the generated
	// all.go blank import).
	_ "github.com/ze-software/ze/internal/component/radius/yang"
)

func TestRadiusAuthMethodEnum(t *testing.T) {
	loader := newTestLoader(t)
	v := yang.NewValidator(loader)

	// validateMethod returns only the enum errors attributable to the
	// auth-method leaf, so a mandatory-field error from another module that also
	// defines a `system` container cannot mask or fake the result.
	validateMethod := func(method string) []yang.ValidationError {
		tree := map[string]any{
			"authentication": map[string]any{
				"radius": map[string]any{
					"server": map[string]any{
						"10.0.0.1": map[string]any{"address": "10.0.0.1", "key": "secret"},
					},
					"auth-method": method,
				},
			},
		}
		var found []yang.ValidationError
		for _, e := range v.ValidateTreeAllModules("system", tree) {
			if e.Type == yang.ErrTypeEnum && strings.Contains(e.Path, "auth-method") {
				found = append(found, e)
			}
		}
		return found
	}

	// RFC requirement: RFC7950-9.6-1 positive -- each value the ze-radius-conf.yang auth-method enum defines is accepted as a valid enumeration value.
	for _, method := range []string{"pap", "chap"} {
		require.Emptyf(t, validateMethod(method),
			"auth-method %q must be accepted by the ze-radius-conf.yang enum", method)
	}

	// An undefined credential is refused, which proves the enum IS enforced on
	// this path and so gives the acceptances above their meaning. mschapv2 is
	// the realistic typo: RFC 2433 and RFC 2759 are not enrolled and ze builds
	// no such credential.
	//
	// RFC requirement: RFC7950-9.6-1 negative -- a value not defined in the enum ("mschapv2") is rejected with ErrTypeEnum.
	errs := validateMethod("mschapv2")
	require.NotEmpty(t, errs, "an undefined auth-method must be refused at config load")
	assert.Equal(t, yang.ErrTypeEnum, errs[0].Type)
	assert.Contains(t, errs[0].Expected, "pap", "the refusal names the permitted values")
	assert.Contains(t, errs[0].Expected, "chap")
}

// VALIDATES: the ze-radius-conf.yang system/authentication/radius/auth-method
// enum accepts the two EAP credentials plan/spec-radius-admin-eap.md added, and
// that the refusal of an undefined value names all four permitted words. The Go
// side of the same boundary is parseAuthMethod
// (internal/component/radius/config.go), tested by TestExtractConfigAuthMethod.
// PREVENTS: schema-and-runtime drift on the two new values. A word the runtime
// builds an EAP peer for and the schema refuses is a documented value an
// operator cannot configure; a word the schema accepts and the runtime does not
// disables the whole RADIUS backend at build time and drops every operator
// login to the local backend.
//
// This is a separate function from TestRadiusAuthMethodEnum, in a separate
// file, on purpose. That test carries `RFC requirement:` tags, so changing its
// body needs an owner row in test/rfc-changed.md that its author may not write.
// New coverage in a new function needs no such row and loses nothing: the two
// functions together assert every value the enum defines.
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

func TestRadiusAuthMethodEnumCoversTheEapValues(t *testing.T) {
	loader := newTestLoader(t)
	v := yang.NewValidator(loader)

	// Only the enum errors attributable to the auth-method leaf, so a mandatory
	// field error from another module that also defines a `system` container
	// cannot mask or fake the result.
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

	for _, method := range []string{"eap-md5", "eap-mschapv2"} {
		require.Emptyf(t, validateMethod(method),
			"auth-method %q must be accepted by the ze-radius-conf.yang enum", method)
	}

	// The refusal is what gives the acceptances above their meaning: it proves
	// the enum is enforced on this path. "eap-tls" is the realistic wrong value
	// now, because ze's EAP peer implements EAP-TLS and this leaf deliberately
	// does not offer it.
	errs := validateMethod("eap-tls")
	require.NotEmpty(t, errs, "an undefined auth-method must be refused at config load")
	assert.Equal(t, yang.ErrTypeEnum, errs[0].Type)
	for _, word := range []string{"pap", "chap", "eap-md5", "eap-mschapv2"} {
		assert.Contains(t, errs[0].Expected, word, "the refusal names every permitted value")
	}
}

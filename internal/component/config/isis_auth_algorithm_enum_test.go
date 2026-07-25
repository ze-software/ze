// VALIDATES: the ze-isis-conf.yang key-chains/key/algorithm enum accepts every
// authentication algorithm the runtime crypto backend supports, in particular
// hmac-sha-224 (auth type 3, RFC 5310). The docs (docs/guide/isis.md,
// docs/guide/configuration.md) advertise hmac-sha-224 and the runtime
// auth_keystore.go/auth_verify.go accept it; this pins the SCHEMA enum so
// `ze config validate` does not reject a documented, runtime-supported value.
// PREVENTS: the B9 regression where the YANG enum omitted hmac-sha-224 while the
// docs and the crypto backend supported it, so a valid config was rejected at
// validation time (config-vs-schema-vs-docs drift).
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	// Register ze-isis-conf so ValidateTreeAllModules can find the isis section
	// (the live binary registers it via the generated all.go blank import).
	_ "github.com/ze-software/ze/internal/plugins/isis/yang"
)

func TestISISAuthAlgorithmEnumAcceptsAll(t *testing.T) {
	loader := newTestLoader(t)
	reg := yang.NewValidatorRegistry()
	RegisterValidators(reg)
	reg.MergeGlobalCompleteFns()
	v := yang.NewValidator(loader)
	v.SetRegistry(reg)

	// validateAlgo builds an isis subtree whose single key-chain holds one key
	// with the given algorithm and validates it through the same walkTree path
	// `ze config validate` uses. It returns only the errors attributable to the
	// algorithm leaf so an unrelated default-related error cannot mask the result.
	validateAlgo := func(algo string) []yang.ValidationError {
		tree := map[string]any{
			"key-chains": map[string]any{
				"area-key": map[string]any{
					"name": "area-key",
					"key": map[string]any{
						"1": map[string]any{
							"key-id":    "1",
							"algorithm": algo,
							"secret":    "s3cr3t",
						},
					},
				},
			},
		}
		var algoErrs []yang.ValidationError
		for _, e := range v.ValidateTreeAllModules("isis", tree) {
			if e.Type == yang.ErrTypeEnum {
				algoErrs = append(algoErrs, e)
			}
		}
		return algoErrs
	}

	// Every algorithm the runtime backend (auth_keystore.go algoFromString /
	// auth_verify.go) supports must be a valid schema enum value -- including
	// hmac-sha-224, the value the B9 finding flagged as missing.
	//
	// RFC requirement: RFC7950-9.6-1 positive -- each algorithm defined in the ze-isis-conf.yang enum is accepted as a valid enumeration value.
	for _, algo := range []string{
		"cleartext",
		"hmac-md5",
		"hmac-sha-1",
		"hmac-sha-224",
		"hmac-sha-256",
		"hmac-sha-384",
		"hmac-sha-512",
	} {
		require.Emptyf(t, validateAlgo(algo),
			"algorithm %q must be accepted by the ze-isis-conf.yang algorithm enum", algo)
	}

	// A value outside the enum is still rejected -- proves the enum IS enforced on
	// this path, so the acceptances above are meaningful (not a no-op walk).
	//
	// RFC requirement: RFC7950-9.6-1 negative -- a value not defined in the enum ("hmac-sha-999") is rejected with ErrTypeEnum.
	errs := validateAlgo("hmac-sha-999")
	require.NotEmpty(t, errs, "an unknown algorithm must be rejected by the enum")
	assert.Equal(t, yang.ErrTypeEnum, errs[0].Type)
}

package filter_path_asn

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// positionLeaves are the five leaf-lists of a reject-asn list that hold ASNs.
// The curated completion hangs off every one of them, so every one is tested.
var positionLeaves = []string{"direct", "transit", "origin", "indirect", "anywhere"}

// asnLeafPath is the config path of one leaf-list this file completes.
var asnLeafPath = []string{
	"bgp", "policy", "reject-asn", "NO-TRANSIT", "transit",
}

// completionsFor drives the real editor entry point. Complete is what the
// config editor calls for every keystroke, so a test that goes through it
// proves an operator reaches these values, and a test over curatedASNValues
// alone would not.
func completionsFor(t *testing.T, input string) []cli.Completion {
	t.Helper()
	completer := cli.NewCompleter()
	require.NotNil(t, completer)
	return completer.Complete(input, nil)
}

// TestCompletionOffersKnownTransitASNs proves the curated table reaches the
// operator, with the network name beside each ASN, through the editor's own
// completion path.
//
// VALIDATES: AC-41, the offering half. Every curated ASN is offered on EACH of
// the five position leaf-lists, and each carries its network name.
// PREVENTS: a ze:validate name, a CompleteFn and a curated table that all exist
// and never meet, which is what the tree held before 2026-09-03: NewCompleter
// built its registry without MergeGlobalCompletions, so no plugin-registered
// CompleteFn was ever consulted. Driving all five leaves also prevents the
// ze:validate line being dropped from one of them, which no other test would see.
func TestCompletionOffersKnownTransitASNs(t *testing.T) {
	for _, leaf := range positionLeaves {
		t.Run(leaf, func(t *testing.T) {
			completions := completionsFor(t, "set bgp policy reject-asn NO-TRANSIT "+leaf+" ")
			offered := make(map[string]string, len(completions))
			for _, completion := range completions {
				offered[completion.Text] = completion.Description
			}

			require.Len(t, offered, len(curatedTransitFree),
				"every curated ASN is offered and nothing else is")

			for _, network := range curatedTransitFree {
				value := textbuf.StringUint32(network.asn)
				description, found := offered[value]
				require.True(t, found, "AS%d is in the curated table and must be offered", network.asn)
				assert.Contains(t, description, network.name,
					"AS%d is offered without naming its network", network.asn)
				if network.contested {
					assert.Contains(t, description, "contested",
						"AS%d is contested and the dropdown must say so", network.asn)
				}
			}
		})
	}
}

// TestCompletionIsNeverAConstraint proves the curated table SUGGESTS and does
// not decide. It is the guard-shaped failure this design exists to avoid: a
// CompleteFn that quietly becomes a ValidateFn would drop an operator's
// legitimate ASN with nothing in the config to point at.
//
// VALIDATES: AC-41, the acceptance half, at three levels. The registry entry
// holds no ValidateFn; the editor's value gate accepts an ASN the table has
// never heard of; and the full module walk, the one path that DOES run custom
// validators over bgp, reports nothing for it.
// PREVENTS: a suggestion hardening into a rule.
func TestCompletionIsNeverAConstraint(t *testing.T) {
	const unlisted = "64512" // a private-use ASN, in no transit-free set

	t.Run("the registry entry refuses nothing", func(t *testing.T) {
		registry := configyang.NewValidatorRegistry()
		config.RegisterValidators(registry)
		registry.MergeGlobalCompletions()

		validator := registry.Get(curatedValidatorName)
		require.NotNil(t, validator, "the merge must create the completion-only entry")
		require.NotNil(t, validator.CompleteFn, "the entry exists to complete")
		assert.Nil(t, validator.ValidateFn,
			"a ValidateFn here would turn every suggestion into a rule")
	})

	t.Run("the editor accepts an unlisted ASN", func(t *testing.T) {
		completer := cli.NewCompleter()
		require.NoError(t, completer.ValidateValueAtPath(asnLeafPath, unlisted))
	})

	t.Run("the module walk reports nothing for an unlisted ASN", func(t *testing.T) {
		body := `        reject-asn NO-TRANSIT {
            transit [ ` + unlisted + ` ]
        }`
		tree, err := config.ParseTreeWithYANG(policyOnly(body), nil)
		require.NoError(t, err)

		validator, err := config.YANGValidatorWithPlugins(nil)
		require.NoError(t, err)

		container := tree.GetContainer("bgp")
		require.NotNil(t, container)

		var messages []string
		for _, ve := range validator.ValidateTreeAllModules("bgp", container.ToMap()) {
			messages = append(messages, ve.Path+": "+ve.Message)
		}
		assert.Empty(t, messages, "an ASN outside the curated table is a valid ASN")
	})
}

// TestCompletionOffersPositionKeys proves an operator meets the position
// vocabulary without reading the YANG, and meets what each leaf covers.
//
// VALIDATES: AC-52. All six leaf-lists are offered inside a reject-asn list that
// holds none of them yet, regex among them, each with the help text its YANG
// description declares.
// PREVENTS: a leaf added to the YANG and never surfaced, so the only way to
// learn the vocabulary is to read the module.
func TestCompletionOffersPositionKeys(t *testing.T) {
	completions := completionsFor(t, "set bgp policy reject-asn NO-TRANSIT ")

	described := make(map[string]string, len(completions))
	for _, completion := range completions {
		described[completion.Text] = completion.Description
	}

	for _, key := range append(append([]string{}, positionLeaves...), "regex") {
		help, found := described[key]
		require.True(t, found, "position leaf %q is in the schema and must be offered", key)
		assert.NotEmpty(t, help, "position leaf %q is offered with no help text", key)
	}
	assert.Contains(t, described["regex"], "RE2",
		"the regex leaf must say it takes a pattern")
}

// TestCompletionAnnotatesNothingItDoesNotKnow proves the annotation is silent
// rather than invented for an ASN outside the table.
//
// VALIDATES: AC-25's producer. curatedASNHelp returns the empty string for an
// unknown value, which is the empty annotation column `show` prints.
// PREVENTS: a guessed network name, and an entry omitted rather than printed.
func TestCompletionAnnotatesNothingItDoesNotKnow(t *testing.T) {
	assert.Empty(t, curatedASNHelp("64512"), "an unlisted ASN has no annotation")
	assert.Empty(t, curatedASNHelp("not-a-number"), "a non-numeric value has no annotation")
	assert.Empty(t, curatedASNHelp(""), "an empty value has no annotation")
	assert.Equal(t, "Verizon Business", curatedASNHelp("701"))
}

// TestCompletionRegistersUnderTheNameTheSchemaCarries proves the ze:validate
// name in the YANG and the name this package registers are the same string.
//
// VALIDATES: the one link nothing else checks. The registration is by name, so
// a rename on either side breaks completion in silence.
// PREVENTS: a CompleteFn registered under a name no leaf carries.
func TestCompletionRegistersUnderTheNameTheSchemaCarries(t *testing.T) {
	schema, err := os.ReadFile("yang/ze-filter-path-asn.yang")
	require.NoError(t, err)

	for _, leaf := range positionLeaves {
		declaration := "leaf-list " + leaf + " {"
		require.True(t, strings.Contains(string(schema), declaration),
			"the leaf-list this file completes must still be called %q", leaf)
	}
	assert.Equal(t, len(positionLeaves)+1,
		strings.Count(string(schema), `ze:validate "`+curatedValidatorName+`"`),
		"the five position leaf-lists and nth's own asn leaf-list carry the name this package registers, and regex does not")
}

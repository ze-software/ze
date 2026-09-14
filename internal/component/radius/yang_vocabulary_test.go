// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS admin AAA config
// Related: config.go -- authMethodNames, the one place this vocabulary is spelled
//
// authMethodNames binds each auth-method word to the credential the authenticator
// builds and the EAP Type it runs, and ze-radius-conf.yang decides which words an
// operator can type. Neither side derives from the other, so the two are gated against
// each other here: a word added to the module alone fails the configuration load, and
// a method added to the type alone is one no operator can ask for
// (ai/rules/principles.md).

package radius

import (
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	// The blank import registers ze-radius-conf with the loader. It imports only
	// the modules the loader embeds, so the leaf below resolves in a binary that
	// links nothing else.
	_ "github.com/ze-software/ze/internal/component/radius/yang"
)

// authMethodLeaf is the leaf whose words parseAuthMethod reads.
const authMethodLeaf = "system/authentication/radius/auth-method"

// TestAuthMethodVocabularyMatchesModel compares the words authMethodNames spells with
// the enumeration the module declares, and drives each model word through the parser
// so the binding is proven at the entry point rather than over the table alone.
func TestAuthMethodVocabularyMatchesModel(t *testing.T) {
	// EnumValues FAILS on a leaf that declares no enumeration rather than answering
	// an empty set, so the comparison below never passes over nothing
	// (ai/rules/evidence.md).
	model, err := configyang.EnumValues(authMethodLeaf)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", authMethodLeaf, err)
	}

	if held := authMethodWords(); !slices.Equal(held, model) {
		t.Errorf("the auth methods disagree: the model at %s holds %v and authMethodNames spells %v. "+
			"A word only the model carries fails the configuration load, and a method only the type carries is unreachable",
			authMethodLeaf, model, held)
	}

	for _, word := range model {
		method, err := parseAuthMethod(word)
		if err != nil {
			t.Errorf("the model admits %q and parseAuthMethod refuses it: %v", word, err)
			continue
		}
		if method.String() != word {
			t.Errorf("parseAuthMethod(%q) selected %s, so the word an operator typed is not the credential the request carries", word, method)
		}
	}

	if _, err := parseAuthMethod("no-such-method"); err == nil {
		t.Error("parseAuthMethod accepted a word no leaf holds, so a schema drift would pick a credential the operator did not choose")
	}
}
